package envmerge

import (
	"path/filepath"
	"strings"
	"testing"
)

// subManager builds a Manager over a single "app" namespace in dir with the given
// resolver factory (nil for identity behavior) and injected OS environment.
func subManager(
	t *testing.T, dir string, factory ValueResolverFactory, osEnv map[string]string,
) *Manager {
	t.Helper()
	return managerFor(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		ResolverFactory: factory,
		OSEnvironment:   osEnv,
	})
}

// TestMaterializeSubstitutesInternalReference verifies Materialize composes a
// transitive {{VAR}} chain over the effective environment.
func TestMaterializeSubstitutesInternalReference(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml",
		"scheme: postgresql\nhost: db.local\nurl: \"{{SCHEME}}://{{HOST}}:5432\"\n",
	)

	env, err := subManager(t, dir, nil, nil).Materialize("")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if got, _ := env.Get("URL"); got != "postgresql://db.local:5432" {
		t.Errorf("URL = %q, want postgresql://db.local:5432", got)
	}
}

// TestMaterializeSubstitutesOSReference verifies a {{@VAR}} reference resolves
// against the injected OS environment.
func TestMaterializeSubstitutesOSReference(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "url: \"https://{{@API_HOST}}\"\n")

	env, err := subManager(t, dir, nil, map[string]string{"API_HOST": "api.example"}).
		Materialize("")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if got, _ := env.Get("URL"); got != "https://api.example" {
		t.Errorf("URL = %q, want https://api.example", got)
	}
}

// TestMaterializeOSValueNotSubstituted verifies an OS-only value that looks like a
// reference is unioned verbatim and never substituted.
func TestMaterializeOSValueNotSubstituted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: localhost\n")

	env, err := subManager(t, dir, nil, map[string]string{"WEIRD": "{{HOST}}"}).
		Materialize("")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if got, _ := env.Get("WEIRD"); got != "{{HOST}}" {
		t.Errorf("WEIRD = %q, want the opaque OS value {{HOST}}", got)
	}
}

// TestMaterializeMissingReferenceFails verifies a {{VAR}} naming no variable is
// fatal.
func TestMaterializeMissingReferenceFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "url: \"{{NOPE}}\"\n")

	if _, err := subManager(t, dir, nil, nil).Materialize(""); err == nil {
		t.Fatal("expected a missing-reference error")
	}
}

// TestMaterializeCircularReferenceFails verifies a reference cycle is fatal.
func TestMaterializeCircularReferenceFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "a: \"{{B}}\"\nb: \"{{A}}\"\n")

	_, err := subManager(t, dir, nil, nil).Materialize("")
	if err == nil {
		t.Fatal("expected a circular-reference error")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error = %v, want a circular-reference message", err)
	}
}

// TestGetMaskedShowsTemplate verifies a masked get shows the {{ }} definition
// unchanged, and a revealed get substitutes it.
func TestGetMaskedShowsTemplate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: db.local\nurl: \"{{HOST}}:5432\"\n")
	manager := subManager(t, dir, nil, nil)

	masked, err := manager.Get(GetParams{Key: "url"})
	if err != nil || masked.Value != "{{HOST}}:5432" {
		t.Fatalf("masked Get = %q, %v; want {{HOST}}:5432", masked.Value, err)
	}

	revealed, err := manager.Get(GetParams{Key: "url", Reveal: true})
	if err != nil || revealed.Value != "db.local:5432" {
		t.Fatalf("revealed Get = %q, %v; want db.local:5432", revealed.Value, err)
	}
}

// TestGetRevealIgnoresUnrelatedDangling verifies a dangling reference behind an
// unrelated key does not block a revealed get whose closure excludes it.
func TestGetRevealIgnoresUnrelatedDangling(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml",
		"host: db.local\ngood: \"{{HOST}}\"\nbad: \"secret://missing\"\n",
	)
	factory := &recordingFactory{resolver: fakeResolver{fail: "secret://missing"}}

	entry, err := subManager(t, dir, factory, nil).
		Get(GetParams{Key: "good", Reveal: true})
	if err != nil {
		t.Fatalf("Get(good): %v", err)
	}
	if entry.Value != "db.local" {
		t.Errorf("Value = %q, want db.local", entry.Value)
	}
}

// TestGetRevealDanglingBehindReferenceFails verifies a dangling reference behind a
// referenced variable blocks a revealed get.
func TestGetRevealDanglingBehindReferenceFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml",
		"password: \"secret://missing\"\nurl: \"{{PASSWORD}}\"\n",
	)
	factory := &recordingFactory{resolver: fakeResolver{fail: "secret://missing"}}

	_, err := subManager(t, dir, factory, nil).
		Get(GetParams{Key: "url", Reveal: true})
	if err == nil {
		t.Fatal("expected the referenced secret's resolution failure to block the read")
	}
}

// TestGetRevealMissingReferenceFails verifies a {{VAR}} naming no variable is
// fatal on the reveal path.
func TestGetRevealMissingReferenceFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "url: \"{{NOPE}}\"\n")

	_, err := subManager(t, dir, nil, nil).Get(GetParams{Key: "url", Reveal: true})
	if err == nil {
		t.Fatal("expected a missing-reference error")
	}
}

// TestGetRevealCircularReferenceFails verifies a reference cycle is fatal on the
// reveal path.
func TestGetRevealCircularReferenceFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "a: \"{{B}}\"\nb: \"{{A}}\"\n")

	_, err := subManager(t, dir, nil, nil).Get(GetParams{Key: "a", Reveal: true})
	if err == nil {
		t.Fatal("expected a circular-reference error")
	}
}

// TestDiffMaskedShowsTemplates verifies a masked diff compares {{ }} definitions,
// so a changed template is visible without substitution.
func TestDiffMaskedShowsTemplates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "url: \"{{DEV}}\"\n")
	writeYAML(t, dir, "app.production.yaml", "url: \"{{PROD}}\"\n")

	result, err := subManager(t, dir, nil, nil).Diff(devToProd)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if c, ok := findChange(result.Changed, "URL"); !ok ||
		c.Before != "{{DEV}}" || c.After != "{{PROD}}" {
		t.Errorf("URL change = %+v, ok=%v; want {{DEV}} -> {{PROD}}", c, ok)
	}
}

// TestDiffRevealSubstitutes verifies a revealed diff resolves and substitutes each
// side, so identical templates over differing inputs read as a change.
func TestDiffRevealSubstitutes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: base\nurl: \"{{HOST}}\"\n")
	writeYAML(t, dir, "app.production.yaml", "host: prod\n")

	result, err := subManager(t, dir, nil, nil).Diff(DiffParams{
		EnvironmentA: "development", EnvironmentB: "production", Reveal: true,
	})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if c, ok := findChange(result.Changed, "URL"); !ok ||
		c.Before != "base" || c.After != "prod" {
		t.Errorf("URL change = %+v, ok=%v; want base -> prod", c, ok)
	}
}
