package envmerge

import (
	"os"
	"path/filepath"
	"testing"
)

// setupWorkspace creates a temp directory with one namespace (env/postgres) and
// returns its path. envmerge reads only the namespace overlays, so no other
// files are needed.
func setupWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	envDir := filepath.Join(dir, "env")
	if err := os.MkdirAll(envDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(envDir, "postgres.yaml"),
		[]byte("host: localhost\nport: 5432\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(envDir, "postgres.production.yaml"),
		[]byte("host: prod-db\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	return dir
}

// baseParams builds envmerge.Params for the temp workspace declaring the
// development and production environments.
func baseParams(dir string) Params {
	return Params{
		Includes:     []string{filepath.Join(dir, "env", "postgres")},
		Environments: []string{"development", "production"},
	}
}

// TestNewAppliesStructuralDefaults verifies New applies the delimiter default and
// does not validate an environment that an operation may override.
func TestNewAppliesStructuralDefaults(t *testing.T) {
	t.Parallel()

	manager := managerFor(t, Params{DefaultEnvironment: "undeclared"})
	if manager.params.Settings.Delimiter != "," {
		t.Errorf("Delimiter = %q, want , (default)", manager.params.Settings.Delimiter)
	}
}

// TestNewPerformsNoNamespaceIO verifies construction reads no namespace files: New
// succeeds even when the include's base file does not exist.
func TestNewPerformsNoNamespaceIO(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if _, err := New(Params{
		Includes:     []string{filepath.Join(dir, "missing")},
		Environments: []string{"development"},
	}); err != nil {
		t.Errorf("New performed namespace I/O: %v", err)
	}
}

// TestNewDoesNotOpenResolver verifies construction never invokes the resolver
// factory; a resolver is opened only when an operation needs it.
func TestNewDoesNotOpenResolver(t *testing.T) {
	t.Parallel()

	factory := &recordingFactory{resolver: fakeResolver{}}
	if _, err := New(Params{
		Environments:    []string{"development"},
		ResolverFactory: factory,
	}); err != nil {
		t.Fatalf("New: %v", err)
	}
	if factory.calls != 0 {
		t.Errorf("New opened %d resolver(s), want 0", factory.calls)
	}
}

// TestNormalizeEnvironment verifies each operation defaults, falls back, and
// validates the environment it uses, and that an explicit environment supersedes
// an unrelated configured default.
func TestNormalizeEnvironment(t *testing.T) {
	t.Parallel()

	t.Run("empty uses configured default", func(t *testing.T) {
		t.Parallel()
		m := managerFor(t, Params{DefaultEnvironment: "production"})
		got, err := m.normalizeEnvironment("")
		if err != nil || got != "production" {
			t.Fatalf("normalizeEnvironment(\"\") = %q, %v; want production", got, err)
		}
	})
	t.Run("empty default falls back to first declared", func(t *testing.T) {
		t.Parallel()
		m := managerFor(t, Params{})
		got, err := m.normalizeEnvironment("")
		if err != nil || got != "development" {
			t.Fatalf("normalizeEnvironment(\"\") = %q, %v; want development", got, err)
		}
	})
	t.Run("explicit supersedes unrelated default", func(t *testing.T) {
		t.Parallel()
		m := managerFor(t, Params{DefaultEnvironment: "undeclared"})
		got, err := m.normalizeEnvironment("production")
		if err != nil || got != "production" {
			t.Fatalf("normalizeEnvironment = %q, %v; want production", got, err)
		}
	})
	t.Run("undeclared environment errors", func(t *testing.T) {
		t.Parallel()
		m := managerFor(t, Params{})
		if _, err := m.normalizeEnvironment("ghost"); err == nil {
			t.Error("expected error for undeclared environment")
		}
	})
}

// TestSelectExplicitEnvironment verifies an explicitly selected environment
// resolves to the merged environment, applying only that environment's overlays.
func TestSelectExplicitEnvironment(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	p.DefaultEnvironment = "development"
	res, err := mergeEnv(t, p)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("HOST = %q, want localhost (development overlay absent)", v)
	}
}

// TestSelectDefaultsToFirstEnvironment verifies an empty DefaultEnvironment falls
// back to the first declared environment.
func TestSelectDefaultsToFirstEnvironment(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	res, err := mergeEnv(t, p)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := p.Environments[0]
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("HOST = %q, want localhost (default env %q)", v, want)
	}
}

// TestSelectProductionAppliesOverlay verifies DefaultEnvironment selects that
// environment's overlay (as diff relies on, passing each side).
func TestSelectProductionAppliesOverlay(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	p.DefaultEnvironment = "production"
	res, err := mergeEnv(t, p)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "prod-db" {
		t.Errorf("HOST = %q, want prod-db", v)
	}
}

// TestSelectUndeclaredEnvironmentFails verifies an undeclared environment fails.
func TestSelectUndeclaredEnvironmentFails(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	p.DefaultEnvironment = "nope"
	if _, err := mergeEnv(t, p); err == nil {
		t.Error("expected error for undeclared environment")
	}
}
