package envmerge

import (
	"path/filepath"
	"testing"
)

// osManager builds a Manager over a single "app" namespace in dir with the given
// injected OS-environment snapshot and overload setting.
func osManager(
	t *testing.T, dir string, osEnv map[string]string, overload bool,
) *Manager {
	t.Helper()
	return managerFor(t, Params{
		Includes:      []string{filepath.Join(dir, "app")},
		OSEnvironment: osEnv,
		Settings:      Settings{Overload: overload},
	})
}

// TestGetOSOverride verifies a masked get returns the OS value for a namespace
// key the OS also sets, and records the OS environment as the winning source
// shadowing the file.
func TestGetOSOverride(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: file-value\n")

	manager := osManager(t, dir, map[string]string{"HOST": "os-value"}, false)
	entry, err := manager.Get(GetParams{Key: "host"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.Value != "os-value" {
		t.Errorf("Value = %q, want os-value (OS overrides file)", entry.Value)
	}
	if entry.Origin.Winner.File != osSource {
		t.Errorf("Winner.File = %q, want %q", entry.Origin.Winner.File, osSource)
	}
	if len(entry.Origin.Shadowed) != 1 ||
		filepath.Base(entry.Origin.Shadowed[0].File) != "app.yaml" {
		t.Errorf("Shadowed = %v, want [app.yaml]", entry.Origin.Shadowed)
	}
}

// TestGetOverloadKeepsFile verifies overload lets the namespace value win over an
// OS value, while still recording the OS source as shadowed.
func TestGetOverloadKeepsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: file-value\n")

	manager := osManager(t, dir, map[string]string{"HOST": "os-value"}, true)
	entry, err := manager.Get(GetParams{Key: "host"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.Value != "file-value" {
		t.Errorf("Value = %q, want file-value (overload keeps file)", entry.Value)
	}
	if filepath.Base(entry.Origin.Winner.File) != "app.yaml" {
		t.Errorf("Winner.File = %q, want app.yaml", entry.Origin.Winner.File)
	}
	if len(entry.Origin.Shadowed) != 1 ||
		entry.Origin.Shadowed[0].File != osSource {
		t.Errorf("Shadowed = %v, want [%q]", entry.Origin.Shadowed, osSource)
	}
}

// TestGetIgnoresOSOnlyKey verifies get enumerates namespace keys only: an OS-only
// key is not gettable.
func TestGetIgnoresOSOnlyKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: file-value\n")

	manager := osManager(t, dir, map[string]string{"OS_ONLY": "x"}, false)
	if _, err := manager.Get(GetParams{Key: "os_only"}); err == nil {
		t.Error("expected OS-only key to be absent from a get")
	}
}

// TestExplainOSOverrideSource verifies masked explain shows the OS value as a
// config value sourced from the OS environment, with the file shadowed.
func TestExplainOSOverrideSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: file-value\n")

	manager := osManager(t, dir, map[string]string{"HOST": "os-value"}, false)
	explanation, err := manager.Explain(ExplainParams{Key: "host"})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	entry := explanation.Entries[0]
	if entry.Literal != "os-value" {
		t.Errorf("Literal = %q, want os-value", entry.Literal)
	}
	if entry.Origin.Winner.File != osSource {
		t.Errorf("Winner.File = %q, want %q", entry.Origin.Winner.File, osSource)
	}
	if entry.Resolution.Kind != KindConfigValue ||
		entry.Resolution.Severity != SeverityOK {
		t.Errorf("Resolution = %+v, want config/ok", entry.Resolution)
	}
	if entry.Resolution.HasResolved {
		t.Error("masked explain must not carry a resolved value")
	}
}

// TestExplainOSOverrideRevealed verifies --reveal surfaces the OS value in the
// resolution, mirroring a plain config value.
func TestExplainOSOverrideRevealed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: file-value\n")

	manager := osManager(t, dir, map[string]string{"HOST": "os-value"}, false)
	explanation, err := manager.Explain(ExplainParams{Key: "host", Reveal: true})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	res := explanation.Entries[0].Resolution
	if !res.HasResolved || res.Resolved != "os-value" {
		t.Errorf("Resolved = %q (has=%v), want os-value", res.Resolved, res.HasResolved)
	}
}

// TestMaterializeUnionsOSKeys verifies Materialize composes the complete
// effective environment: namespace values, OS overrides, and OS-only keys.
func TestMaterializeUnionsOSKeys(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: file-value\nport: 5432\n")

	manager := osManager(t, dir, map[string]string{
		"HOST":    "os-value",
		"OS_ONLY": "extra",
	}, false)
	env, err := manager.Materialize("")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if got, _ := env.Get("HOST"); got != "os-value" {
		t.Errorf("HOST = %q, want os-value (OS wins)", got)
	}
	if got, _ := env.Get("PORT"); got != "5432" {
		t.Errorf("PORT = %q, want 5432 (namespace-only)", got)
	}
	if got, ok := env.Get("OS_ONLY"); !ok || got != "extra" {
		t.Errorf("OS_ONLY = %q (ok=%v), want extra (OS-only unioned)", got, ok)
	}
}

// TestMaterializeOverloadKeepsFile verifies overload keeps namespace values over
// OS values while still unioning OS-only keys.
func TestMaterializeOverloadKeepsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: file-value\n")

	manager := osManager(t, dir, map[string]string{
		"HOST":    "os-value",
		"OS_ONLY": "extra",
	}, true)
	env, err := manager.Materialize("")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if got, _ := env.Get("HOST"); got != "file-value" {
		t.Errorf("HOST = %q, want file-value (overload keeps file)", got)
	}
	if got, ok := env.Get("OS_ONLY"); !ok || got != "extra" {
		t.Errorf("OS_ONLY = %q (ok=%v), want extra", got, ok)
	}
}

// TestMaterializeOSValueIsOpaque verifies an OS value that looks like a reference
// is injected verbatim and never sent to the resolver.
func TestMaterializeOSValueIsOpaque(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: file-value\n")

	// A revealing factory would fail every reference; an opaque OS value must not
	// reach it, so materialization succeeds with the reference kept verbatim.
	manager := managerFor(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		OSEnvironment:   map[string]string{"HOST": "secret://group/key"},
		ResolverFactory: &recordingFactory{resolver: fakeResolver{failAll: true}},
	})
	env, err := manager.Materialize("")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if got, _ := env.Get("HOST"); got != "secret://group/key" {
		t.Errorf("HOST = %q, want the opaque OS value verbatim", got)
	}
}

// TestDiffAppliesOSOverride verifies diff compares OS-overridden values, so a key
// the OS pins to one value on both sides is not reported as changed.
func TestDiffAppliesOSOverride(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: base\n")
	writeYAML(t, dir, "app.production.yaml", "host: prod\n")

	manager := osManager(t, dir, map[string]string{"HOST": "os-value"}, false)
	result, err := manager.Diff("development", "production")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(result.Changed) != 0 {
		t.Errorf("Changed = %v, want none (OS pins HOST on both sides)", result.Changed)
	}
}
