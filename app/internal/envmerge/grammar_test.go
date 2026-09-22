package envmerge

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/status"
)

// customPatternManager builds a Manager over a single "app" namespace in dir whose
// reference syntax is redefined to ${VAR}, so a lifecycle test exercises a custom
// grammar through the real merge pipeline.
func customPatternManager(
	t *testing.T, dir string, osEnv map[string]string,
) *Manager {
	t.Helper()
	return managerFor(t, Params{
		Includes:      []string{filepath.Join(dir, "app")},
		OSEnvironment: osEnv,
		Settings: Settings{
			ReferencePattern: `\$\{([^}]*)\}`,
		},
	})
}

// TestNewGrammarDefaults verifies an empty pattern falls back to the built-in
// grammar, tokenizing {{VAR}} exactly as the default constant does.
func TestNewGrammarDefaults(t *testing.T) {
	t.Parallel()

	g, err := newGrammar("")
	if err != nil {
		t.Fatalf("newGrammar(default): %v", err)
	}

	got := g.tokenize("{{SCHEME}}://{{HOST}}")
	want := []token{
		{tokenReference, "SCHEME"},
		{tokenLiteral, "://"},
		{tokenReference, "HOST"},
	}
	if len(got) != len(want) {
		t.Fatalf("tokenize = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("token[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestNewGrammarCustomPattern verifies a workspace can redefine the reference
// syntax and the engine composes values through the custom grammar.
func TestNewGrammarCustomPattern(t *testing.T) {
	t.Parallel()

	// Redefine references as ${VAR}.
	g, err := newGrammar(`\$\{([^}]*)\}`)
	if err != nil {
		t.Fatalf("newGrammar(custom): %v", err)
	}

	got := g.tokenize("${SCHEME}://${HOST}")
	want := []token{
		{tokenReference, "SCHEME"},
		{tokenLiteral, "://"},
		{tokenReference, "HOST"},
	}
	if len(got) != len(want) {
		t.Fatalf("tokenize = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("token[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	// The default {{VAR}} syntax is inert under the custom grammar.
	if refs := g.hasReferences("{{SCHEME}}"); refs {
		t.Error("default {{VAR}} should not tokenize under a custom grammar")
	}
}

// TestNewGrammarInvalidPattern verifies an uncompilable pattern is a clear error
// naming the reference pattern, so it fails at config time rather than at use.
func TestNewGrammarInvalidPattern(t *testing.T) {
	t.Parallel()

	if _, err := newGrammar(`(unterminated`); err == nil ||
		!strings.Contains(err.Error(), "reference pattern") {
		t.Errorf("pattern error = %v, want a reference pattern error", err)
	}
}

// TestNewGrammarMissingCaptureGroup verifies a pattern without a capture group is
// rejected, since the engine reads the variable name from the first group.
func TestNewGrammarMissingCaptureGroup(t *testing.T) {
	t.Parallel()

	if _, err := newGrammar(`\{\{[^}]*\}\}`); err == nil ||
		!strings.Contains(err.Error(), "capture group") {
		t.Errorf("error = %v, want a missing-capture-group error", err)
	}
}

// TestCustomGrammarResolvesThroughManager verifies a Manager built with a custom
// pattern substitutes references written in the custom syntax, end to end.
func TestCustomGrammarResolvesThroughManager(t *testing.T) {
	t.Parallel()

	m, err := New(Params{
		Settings: Settings{ReferencePattern: `\$\{([^}]*)\}`},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	engine := newSymbolSubstituter(
		m.grammar,
		mapSymbols(
			map[string]string{"HOST": "db.local", "URL": "postgresql://${HOST}"},
			map[string]Origin{},
		),
		noGetenv,
		false,
	)
	got, err := engine.resolve("URL")
	if err != nil {
		t.Fatalf("resolve(URL): %v", err)
	}
	if want := "postgresql://db.local"; got != want {
		t.Errorf("resolve(URL) = %q, want %q", got, want)
	}
}

// TestNewManagerInvalidPatternFails verifies an invalid reference pattern fails at
// Manager construction — config time — rather than deferring to an operation.
func TestNewManagerInvalidPatternFails(t *testing.T) {
	t.Parallel()

	if _, err := New(Params{
		Settings: Settings{ReferencePattern: `(unterminated`},
	}); err == nil {
		t.Error("New with an invalid reference pattern should fail")
	}
}

// TestMaterializeCustomPatternLifecycle verifies a custom grammar flows all the
// way through the merge → materialize → substitute pipeline: ${VAR} composes a
// transitive chain, resolves an OS-only value against the environment, and the
// built-in {{VAR}} syntax is inert (carried through literally).
func TestMaterializeCustomPatternLifecycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml",
		"scheme: postgresql\n"+
			"host: db.local\n"+
			"url: \"${SCHEME}://${HOST}:5432\"\n"+
			"api: \"https://${API_HOST}\"\n"+
			"inert: \"{{SCHEME}}\"\n",
	)

	env := materializeEnv(
		t, customPatternManager(t, dir, map[string]string{"API_HOST": "api.example"}), "",
	)

	if got, _ := env.Get("URL"); got != "postgresql://db.local:5432" {
		t.Errorf("URL = %q, want postgresql://db.local:5432", got)
	}
	if got, _ := env.Get("API"); got != "https://api.example" {
		t.Errorf("API = %q, want https://api.example", got)
	}
	if got, _ := env.Get("INERT"); got != "{{SCHEME}}" {
		t.Errorf("INERT = %q, want the untouched {{SCHEME}}", got)
	}
}

// TestGetCustomPatternLifecycle verifies a revealed Get resolves a custom-syntax
// reference through its transitive dependency closure.
func TestGetCustomPatternLifecycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: db.local\nurl: \"postgresql://${HOST}\"\n")

	entry, err := customPatternManager(t, dir, nil).Get(
		GetParams{Key: "url", Reveal: true},
	)
	if err != nil {
		t.Fatalf("Get(url): %v", err)
	}
	if entry.Value != "postgresql://db.local" {
		t.Errorf("URL = %q, want postgresql://db.local", entry.Value)
	}
}

// TestExplainCustomPatternLifecycle verifies masked explain classifies a
// custom-syntax value as a variable substitution showing its template, and a
// dangling custom reference is diagnosed as UNRESOLVED_VARIABLE without aborting.
func TestExplainCustomPatternLifecycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: db.local\nurl: \"${HOST}\"\nbad: \"${NOPE}\"\n")
	manager := customPatternManager(t, dir, nil)

	exp, err := manager.Explain(ExplainParams{})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}

	url, _ := findExplanation(exp, "URL")
	if url.Resolution.Kind != KindVariableSubstitution {
		t.Errorf("URL kind = %q, want variable", url.Resolution.Kind)
	}
	if url.Literal != "${HOST}" {
		t.Errorf("URL literal = %q, want ${HOST}", url.Literal)
	}
	if url.Resolution.HasResolved {
		t.Errorf("masked URL leaked a value: %+v", url.Resolution)
	}

	bad, _ := findExplanation(exp, "BAD")
	if bad.Resolution.Code != status.UnresolvedVariableReference {
		t.Errorf("BAD code = %q, want UNRESOLVED_VARIABLE", bad.Resolution.Code)
	}
}
