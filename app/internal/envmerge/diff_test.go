package envmerge

import (
	"path/filepath"
	"strings"
	"testing"
)

// diffManager builds a Manager over a single "app" namespace in dir declaring the
// development and production environments, with an optional resolver factory so a
// test can prove Diff never opens one.
func diffManager(t *testing.T, dir string, factory ValueResolverFactory) *Manager {
	t.Helper()
	return managerFor(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		ResolverFactory: factory,
	})
}

// findChange returns the change with the given key and whether it was present.
func findChange(changes []Change, key string) (Change, bool) {
	for _, c := range changes {
		if c.Key == key {
			return c, true
		}
	}
	return Change{}, false
}

// devToProd is the common masked diff selecting development against production.
var devToProd = DiffParams{EnvironmentA: "development", EnvironmentB: "production"}

// TestDiffValidatesEnvironmentsBeforeIO verifies an undeclared environment on
// either side errors, and that validation happens before namespace I/O by using
// a manager whose base file does not exist.
func TestDiffValidatesEnvironmentsBeforeIO(t *testing.T) {
	t.Parallel()

	manager := managerFor(t, Params{
		Includes:     []string{filepath.Join(t.TempDir(), "missing")},
		Environments: []string{"development", "production"},
	})

	if _, err := manager.Diff(DiffParams{
		EnvironmentA: "ghost", EnvironmentB: "production",
	}); err == nil {
		t.Error("expected error for undeclared environment A")
	}
	if _, err := manager.Diff(DiffParams{
		EnvironmentA: "development", EnvironmentB: "ghost",
	}); err == nil {
		t.Error("expected error for undeclared environment B")
	}
}

// TestDiffClassifiesChanges verifies added, removed, and changed keys are
// reported with their literal values on each side.
func TestDiffClassifiesChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: base\nkept: same\n")
	writeYAML(t, dir, "app.development.yaml", "only_dev: yes\n")
	writeYAML(t, dir, "app.production.yaml", "host: prod\nonly_prod: yes\n")

	result, err := diffManager(t, dir, nil).Diff(devToProd)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	if result.EnvironmentA != "development" || result.EnvironmentB != "production" {
		t.Errorf(
			"environments = %q, %q; want development, production",
			result.EnvironmentA, result.EnvironmentB,
		)
	}

	if c, ok := findChange(result.Changed, "HOST"); !ok ||
		c.Before != "base" || c.After != "prod" {
		t.Errorf("HOST change = %+v, ok=%v; want base -> prod", c, ok)
	}
	if c, ok := findChange(result.Removed, "ONLY_DEV"); !ok || c.Before != "yes" {
		t.Errorf("ONLY_DEV removed = %+v, ok=%v; want before=yes", c, ok)
	}
	if c, ok := findChange(result.Added, "ONLY_PROD"); !ok || c.After != "yes" {
		t.Errorf("ONLY_PROD added = %+v, ok=%v; want after=yes", c, ok)
	}
	if _, ok := findChange(result.Changed, "KEPT"); ok {
		t.Error("KEPT should not be reported as changed")
	}
}

// TestDiffSortsResults verifies added, removed, and changed slices are sorted by
// key.
func TestDiffSortsResults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "b: 1\nd: 1\nf: 1\n")
	writeYAML(t, dir, "app.production.yaml", "b: 2\nd: 2\nf: 2\n")

	result, err := diffManager(t, dir, nil).Diff(devToProd)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	want := []string{"B", "D", "F"}
	if len(result.Changed) != len(want) {
		t.Fatalf(
			"Changed len = %d, want %d (%+v)",
			len(result.Changed), len(want), result.Changed,
		)
	}
	for i, key := range want {
		if result.Changed[i].Key != key {
			t.Errorf("Changed[%d].Key = %q, want %q", i, result.Changed[i].Key, key)
		}
	}
}

// TestDiffIdenticalEnvironments verifies diffing an environment against itself
// yields an empty result.
func TestDiffIdenticalEnvironments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: base\n")

	result, err := diffManager(t, dir, nil).Diff(DiffParams{
		EnvironmentA: "development", EnvironmentB: "development",
	})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(result.Added) != 0 || len(result.Removed) != 0 || len(result.Changed) != 0 {
		t.Errorf("expected empty diff, got %+v", result)
	}
}

// TestDiffNeverOpensResolver verifies Diff compares references without opening a
// resolver: a changed reference is reported even though the factory is never
// invoked and both references would decrypt to equal plaintext.
func TestDiffNeverOpensResolver(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "token: secret://dev\n")
	writeYAML(t, dir, "app.production.yaml", "token: secret://prod\n")

	// The factory would resolve both references to the same plaintext, so a
	// resolving diff would hide the change. It must never be called.
	factory := &recordingFactory{resolver: fakeResolver{
		values: map[string]string{"secret://dev": "same", "secret://prod": "same"},
	}}

	result, err := diffManager(t, dir, factory).Diff(devToProd)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if factory.calls != 0 {
		t.Errorf("Diff opened %d resolver(s), want 0", factory.calls)
	}
	if c, ok := findChange(result.Changed, "TOKEN"); !ok ||
		c.Before != "secret://dev" || c.After != "secret://prod" {
		t.Errorf(
			"TOKEN change = %+v, ok=%v; want secret://dev -> secret://prod", c, ok,
		)
	}
}

// TestDiffDanglingReferenceCompares verifies a reference to a missing secret is
// compared as a declaration without error, since Diff never resolves it.
func TestDiffDanglingReferenceCompares(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "token: secret://missing\n")
	writeYAML(t, dir, "app.production.yaml", "token: secret://also-missing\n")

	factory := &recordingFactory{resolver: fakeResolver{failAll: true}}
	result, err := diffManager(t, dir, factory).Diff(devToProd)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if c, ok := findChange(result.Changed, "TOKEN"); !ok ||
		c.Before != "secret://missing" || c.After != "secret://also-missing" {
		t.Errorf("TOKEN change = %+v, ok=%v; want the declared references", c, ok)
	}
}

// TestDiffLiteralNotCanonicalized verifies implicit references and escaped
// reference literals compare exactly as declared, without resolver
// canonicalization.
func TestDiffLiteralNotCanonicalized(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "a: \"group/key\"\nb: \"\\\\secret://x\"\n")
	writeYAML(t, dir, "app.production.yaml", "a: \"group/other\"\n")

	result, err := diffManager(t, dir, nil).Diff(devToProd)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if c, ok := findChange(result.Changed, "A"); !ok ||
		c.Before != "group/key" || c.After != "group/other" {
		t.Errorf("A change = %+v, ok=%v; want group/key -> group/other", c, ok)
	}
	if _, ok := findChange(result.Changed, "B"); ok {
		t.Error("escaped literal B should compare equal to itself")
	}
}

// TestDiffListDelimiterFails verifies a list item containing the configured
// delimiter fails literal rendering, and the error does not expose the item
// value.
func TestDiffListDelimiterFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "hosts:\n  - \"a,b\"\n")

	_, err := diffManager(t, dir, nil).Diff(devToProd)
	if err == nil {
		t.Fatal("expected a literal-render error for an item containing the delimiter")
	}
	if got := err.Error(); strings.Contains(got, "a,b") {
		t.Errorf("error leaked the item value: %q", got)
	}
}

// TestDiffNamespaceFailureNoPartial verifies a malformed namespace on one side
// aborts the whole comparison rather than returning a partial result.
func TestDiffNamespaceFailureNoPartial(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: base\n")
	writeYAML(t, dir, "app.production.yaml", "host:\n  - key: val\n")

	if _, err := diffManager(t, dir, nil).Diff(devToProd); err == nil {
		t.Error("expected a fatal error for the malformed production overlay")
	}
}
