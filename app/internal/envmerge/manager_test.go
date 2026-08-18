package envmerge

import (
	"path/filepath"
	"testing"
)

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
