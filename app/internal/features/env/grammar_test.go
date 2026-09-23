package env

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/shared/status"
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

	engine := m.newSubstituter(mapSymbols(
		map[string]string{"HOST": "db.local", "URL": "postgresql://${HOST}"},
		map[string]Origin{},
	))
	got, err := engine.Resolve("URL")
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
