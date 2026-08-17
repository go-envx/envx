package envmerge

import (
	"path/filepath"
	"strings"
	"testing"
)

// fakeDiagnoser implements both ValueResolver and ValueDiagnoser for testing the
// Explain flow. It classifies "secret://" values as references, resolves known
// ones to plaintext under reveal, and reports designated statuses without
// aborting. Plain values are config values.
type fakeDiagnoser struct {
	// reveal controls whether a successful resolution retains plaintext.
	reveal bool
	// values maps a resolvable secret reference to its plaintext.
	values map[string]string
	// warn is a reference reported at warning severity.
	warn string
	// fail is a reference reported at error severity.
	fail string
}

// Resolve satisfies ValueResolver; Explain uses only Diagnose, so Resolve simply
// dereferences known references and passes everything else through.
func (f fakeDiagnoser) Resolve(value, _ string) (string, error) {
	if v, ok := f.values[value]; ok {
		return v, nil
	}
	return value, nil
}

// Diagnose classifies value and reports its non-fatal outcome, retaining
// plaintext only when reveal is set.
func (f fakeDiagnoser) Diagnose(value, _ string) Resolution {
	if !strings.HasPrefix(value, "secret://") {
		res := Resolution{Kind: KindConfigValue, Severity: SeverityOK, Code: codeOK}
		if f.reveal {
			res.Resolved = value
			res.HasResolved = true
		}
		return res
	}
	if value == f.fail {
		return Resolution{
			Kind: KindSecretReference, Severity: SeverityError,
			Code: "SECRET_NOT_FOUND", Message: "dangling reference",
		}
	}
	if value == f.warn {
		return Resolution{
			Kind: KindSecretReference, Severity: SeverityWarning,
			Code: "PRIVATE_KEY_UNAVAILABLE", Message: "no key",
		}
	}
	res := Resolution{Kind: KindSecretReference, Severity: SeverityOK, Code: codeOK}
	if f.reveal {
		res.Resolved = f.values[value]
		res.HasResolved = true
	}
	return res
}

// diagnoserFactory returns a fakeDiagnoser reflecting the requested reveal
// policy, recording how many resolvers it opened.
type diagnoserFactory struct {
	// calls counts how many times Resolver was invoked.
	calls int
	// base is the diagnoser template whose reveal flag is set per call.
	base fakeDiagnoser
}

// Resolver returns a fresh diagnoser bound to the reveal policy.
func (f *diagnoserFactory) Resolver(reveal bool) (ValueResolver, error) {
	f.calls++
	d := f.base
	d.reveal = reveal
	return d, nil
}

// explainManager builds a Manager over a single "app" namespace in dir with the
// given resolver factory.
func explainManager(t *testing.T, dir string, factory ValueResolverFactory) *Manager {
	t.Helper()
	return managerFor(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		ResolverFactory: factory,
	})
}

// TestExplainAllKeysSorted verifies an empty key explains every winning key in
// sorted order, carrying literals and provenance.
func TestExplainAllKeysSorted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: localhost\nport: 5432\n")
	writeYAML(t, dir, "app.development.yaml", "host: dev-db.local\n")

	manager := explainManager(t, dir, &diagnoserFactory{})
	exp, err := manager.Explain(ExplainParams{})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}

	if len(exp.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(exp.Entries))
	}
	if exp.Entries[0].Key != "HOST" || exp.Entries[1].Key != "PORT" {
		t.Errorf("entries not sorted: %+v", exp.Entries)
	}
	if exp.Entries[0].Literal != "dev-db.local" {
		t.Errorf("HOST literal = %q, want dev-db.local", exp.Entries[0].Literal)
	}
	if filepath.Base(exp.Entries[0].Origin.Winner.File) != "app.development.yaml" {
		t.Errorf("HOST winner = %q", exp.Entries[0].Origin.Winner.File)
	}
	if len(exp.Entries[0].Origin.Shadowed) != 1 {
		t.Errorf("HOST shadow history = %+v", exp.Entries[0].Origin.Shadowed)
	}
}

// TestExplainSpecificKeyCaseInsensitive verifies a non-empty key explains just
// that key, matched case-insensitively.
func TestExplainSpecificKeyCaseInsensitive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: localhost\nport: 5432\n")

	manager := explainManager(t, dir, &diagnoserFactory{})
	exp, err := manager.Explain(ExplainParams{Key: "host"})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if len(exp.Entries) != 1 || exp.Entries[0].Key != "HOST" {
		t.Fatalf("expected single HOST entry, got %+v", exp.Entries)
	}
}

// TestExplainMissingKey verifies an unknown key is an operation error.
func TestExplainMissingKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: localhost\n")

	manager := explainManager(t, dir, &diagnoserFactory{})
	if _, err := manager.Explain(ExplainParams{Key: "nope"}); err == nil {
		t.Fatal("expected error for missing key")
	}
}

// TestExplainPerKeyFailureDoesNotAbort verifies a per-key resolution failure is
// reported through its Resolution and reflected in the summary without aborting.
func TestExplainPerKeyFailureDoesNotAbort(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "good: plain\nbad: secret://missing\n")

	factory := &diagnoserFactory{base: fakeDiagnoser{fail: "secret://missing"}}
	manager := explainManager(t, dir, factory)
	exp, err := manager.Explain(ExplainParams{})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}

	if exp.Summary.Errors != 1 || exp.Summary.Severity() != SeverityError {
		t.Errorf("summary = %+v, severity %q", exp.Summary, exp.Summary.Severity())
	}
	bad, ok := findExplanation(exp, "BAD")
	if !ok || bad.Resolution.Code != "SECRET_NOT_FOUND" {
		t.Errorf("BAD resolution = %+v", bad.Resolution)
	}
	if bad.Resolution.Kind != KindSecretReference {
		t.Errorf("BAD kind = %q, want secret", bad.Resolution.Kind)
	}
}

// TestExplainRejectsNonDiagnoser verifies a configured resolver that does not
// implement ValueDiagnoser is an operation error.
func TestExplainRejectsNonDiagnoser(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: localhost\n")

	factory := &recordingFactory{resolver: fakeResolver{}}
	manager := explainManager(t, dir, factory)
	if _, err := manager.Explain(ExplainParams{}); err == nil {
		t.Fatal("expected error for a resolver without diagnosis support")
	}
}

// TestExplainNilFactoryIsConfigValue verifies a nil factory classifies every
// value as a plain config value at ok severity.
func TestExplainNilFactoryIsConfigValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: localhost\n")

	manager := explainManager(t, dir, nil)
	exp, err := manager.Explain(ExplainParams{})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if exp.Entries[0].Resolution.Kind != KindConfigValue {
		t.Errorf("kind = %q, want config", exp.Entries[0].Resolution.Kind)
	}
	if exp.Entries[0].Resolution.Severity != SeverityOK {
		t.Errorf("severity = %q, want ok", exp.Entries[0].Resolution.Severity)
	}
}

// TestExplainRevealRetainsResolved verifies reveal retains resolved plaintext
// while a masked call opens a fresh resolver and retains none.
func TestExplainRevealRetainsResolved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "token: secret://ok\n")

	factory := &diagnoserFactory{base: fakeDiagnoser{
		values: map[string]string{"secret://ok": "plaintext"},
	}}
	manager := explainManager(t, dir, factory)

	masked, err := manager.Explain(ExplainParams{})
	if err != nil {
		t.Fatalf("Explain masked: %v", err)
	}
	if masked.Entries[0].Resolution.HasResolved {
		t.Errorf("masked explanation retained plaintext: %+v", masked.Entries[0])
	}

	revealed, err := manager.Explain(ExplainParams{Reveal: true})
	if err != nil {
		t.Fatalf("Explain reveal: %v", err)
	}
	got := revealed.Entries[0].Resolution
	if !got.HasResolved || got.Resolved != "plaintext" {
		t.Errorf("revealed resolution = %+v, want resolved plaintext", got)
	}
	if factory.calls != 2 {
		t.Errorf("expected 2 resolver opens, got %d", factory.calls)
	}
}

// TestExplainRevealResolvedEmpty verifies a resolved empty string is retained
// distinctly from an unresolved value.
func TestExplainRevealResolvedEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "blank:\n")

	factory := &diagnoserFactory{}
	manager := explainManager(t, dir, factory)
	exp, err := manager.Explain(ExplainParams{Reveal: true})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	got := exp.Entries[0].Resolution
	if !got.HasResolved || got.Resolved != "" {
		t.Errorf("resolution = %+v, want resolved empty string", got)
	}
}

// TestExplainListAggregatesWorstSeverity verifies a list's outcome aggregates a
// secret kind and the worst item severity and resolves only when every item
// succeeds.
func TestExplainListAggregatesWorstSeverity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "tokens:\n  - secret://ok\n  - secret://warn\n")

	factory := &diagnoserFactory{base: fakeDiagnoser{
		values: map[string]string{"secret://ok": "a"},
		warn:   "secret://warn",
	}}
	manager := explainManager(t, dir, factory)
	exp, err := manager.Explain(ExplainParams{Reveal: true})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	got := exp.Entries[0].Resolution
	if got.Kind != KindSecretReference {
		t.Errorf("kind = %q, want secret", got.Kind)
	}
	if got.Severity != SeverityWarning {
		t.Errorf("severity = %q, want warning", got.Severity)
	}
	if got.HasResolved {
		t.Errorf("list with an unresolved item should not retain plaintext: %+v", got)
	}
	if exp.Summary.Warnings != 1 {
		t.Errorf("summary warnings = %d, want 1", exp.Summary.Warnings)
	}
}

// findExplanation returns the entry with the given key and whether it exists.
func findExplanation(exp *Explanation, key string) (ExplanationEntry, bool) {
	for i := range exp.Entries {
		if exp.Entries[i].Key == key {
			return exp.Entries[i], true
		}
	}
	return ExplanationEntry{}, false
}
