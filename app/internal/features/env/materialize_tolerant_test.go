package env

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestMaterializeIgnoringErrorsOmitsMissingReference verifies a {{VAR}} naming no
// variable is downgraded to a warning and its key omitted, while unrelated keys
// resolve.
func TestMaterializeIgnoringErrorsOmitsMissingReference(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: db.local\nbroken: \"{{NOPE}}\"\n")

	result, err := subManager(t, dir, nil, nil).
		Materialize(MaterializeParams{IgnoreErrors: true})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	env, warnings := result.Environment, result.Warnings
	if got, ok := env.Get("HOST"); !ok || got != "db.local" {
		t.Errorf("HOST = %q, ok=%v; want db.local", got, ok)
	}
	if _, ok := env.Get("BROKEN"); ok {
		t.Error("BROKEN should be omitted from the environment")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Error(), "BROKEN") {
		t.Errorf("warnings = %v, want one naming BROKEN", warnings)
	}
}

// TestMaterializeIgnoringErrorsOmitsWholeCycle verifies every key in a reference
// cycle is omitted with a warning, while a key outside the cycle resolves.
func TestMaterializeIgnoringErrorsOmitsWholeCycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "a: \"{{B}}\"\nb: \"{{A}}\"\nok: fine\n")

	result, err := subManager(t, dir, nil, nil).
		Materialize(MaterializeParams{IgnoreErrors: true})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	env, warnings := result.Environment, result.Warnings
	if got, ok := env.Get("OK"); !ok || got != "fine" {
		t.Errorf("OK = %q, ok=%v; want fine", got, ok)
	}
	if _, ok := env.Get("A"); ok {
		t.Error("A should be omitted as part of the cycle")
	}
	if _, ok := env.Get("B"); ok {
		t.Error("B should be omitted as part of the cycle")
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want two (one per cycle member)", warnings)
	}
	// Warnings are sorted by key, so A precedes B.
	if !strings.HasPrefix(warnings[0].Error(), "omitting A:") ||
		!strings.HasPrefix(warnings[1].Error(), "omitting B:") {
		t.Errorf("warnings = %v, want sorted A then B", warnings)
	}
}

// TestMaterializeIgnoringErrorsOmitsDanglingSecret verifies a dangling secret
// reference is downgraded to a warning and its key omitted, while sibling keys
// resolve.
func TestMaterializeIgnoringErrorsOmitsDanglingSecret(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml",
		"host: db.local\npassword: \"secret://missing\"\n",
	)
	factory := &recordingFactory{resolver: fakeResolver{fail: "secret://missing"}}

	result, err := subManager(t, dir, factory, nil).
		Materialize(MaterializeParams{IgnoreErrors: true})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	env, warnings := result.Environment, result.Warnings
	if got, ok := env.Get("HOST"); !ok || got != "db.local" {
		t.Errorf("HOST = %q, ok=%v; want db.local", got, ok)
	}
	if _, ok := env.Get("PASSWORD"); ok {
		t.Error("PASSWORD should be omitted from the environment")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Error(), "PASSWORD") {
		t.Errorf("warnings = %v, want one naming PASSWORD", warnings)
	}
}

// TestMaterializeIgnoringErrorsFallsBackToOSValue verifies that under overload a
// broken file value that the OS environment still defines falls back to the OS
// value instead of being omitted, so the file value never clobbers it.
func TestMaterializeIgnoringErrorsFallsBackToOSValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "broken: \"{{MISSING}}\"\n")
	manager := managerFor(t, Params{
		Includes:      []string{filepath.Join(dir, "app")},
		OSEnvironment: map[string]string{"BROKEN": "from-os"},
		Settings:      Settings{Overload: true},
	})

	result, err := manager.Materialize(MaterializeParams{IgnoreErrors: true})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	env, warnings := result.Environment, result.Warnings
	if got, ok := env.Get("BROKEN"); !ok || got != "from-os" {
		t.Errorf("BROKEN = %q, ok=%v; want the OS fallback from-os", got, ok)
	}
	if len(warnings) != 1 ||
		!strings.Contains(warnings[0].Error(), "keeping the value") {
		t.Errorf("warnings = %v, want one noting the environment fallback", warnings)
	}
}

// TestMaterializeIgnoringErrorsResolvesCleanly verifies a fully resolvable
// environment yields every value and no warnings.
func TestMaterializeIgnoringErrorsResolvesCleanly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: db.local\nurl: \"{{HOST}}:5432\"\n")

	result, err := subManager(t, dir, nil, nil).
		Materialize(MaterializeParams{IgnoreErrors: true})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	env, warnings := result.Environment, result.Warnings
	if warnings != nil {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if got, _ := env.Get("URL"); got != "db.local:5432" {
		t.Errorf("URL = %q, want db.local:5432", got)
	}
}

// TestMaterializeIgnoringErrorsStructuralStillFatal verifies a flatten collision
// remains fatal even under the lenient path, because it leaves no salvageable
// environment.
func TestMaterializeIgnoringErrorsStructuralStillFatal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "a_b: 1\na:\n  b: 2\n")

	result, err := subManager(t, dir, nil, nil).
		Materialize(MaterializeParams{IgnoreErrors: true})
	if err == nil {
		t.Fatal("expected a flatten collision to stay fatal")
	}
	if result != nil {
		t.Errorf("result = %v, want nil on a fatal error", result)
	}
}
