package envmerge

import (
	"path/filepath"
	"strings"
	"testing"
)

// recordingFactory is a ValueResolverFactory that records how many resolvers it
// opened and the reveal policy of the last call, returning a caller-supplied
// resolver. It proves each operation opens exactly one fresh resolver and that
// construction opens none.
type recordingFactory struct {
	// calls counts how many times Resolver was invoked.
	calls int
	// reveal records the reveal policy of the most recent call.
	reveal bool
	// resolver is returned to the operation on each call.
	resolver ValueResolver
}

// Resolver records the call and returns the configured resolver.
func (f *recordingFactory) Resolver(reveal bool) (ValueResolver, error) {
	f.calls++
	f.reveal = reveal
	return f.resolver, nil
}

// mutableFactory returns a fresh resolver reflecting its current value on each
// call, so a test can prove no resolver state survives across operations.
type mutableFactory struct {
	// calls counts how many times Resolver was invoked.
	calls int
	// value is the plaintext the returned resolver maps "secret://x" to.
	value string
}

// Resolver returns a fresh resolver bound to the factory's current value.
func (f *mutableFactory) Resolver(bool) (ValueResolver, error) {
	f.calls++
	return fakeResolver{values: map[string]string{"secret://x": f.value}}, nil
}

// managerFor builds a Manager over a single namespace declaring development and
// production, without validating the environment at construction.
func managerFor(t *testing.T, params Params) *Manager {
	t.Helper()
	if params.Environments == nil {
		params.Environments = []string{"development", "production"}
	}
	manager, err := New(params)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return manager
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

// TestMaterializeResolvesEveryWinner verifies Materialize returns a complete
// environment with every winning value resolved, and that All returns a copy.
func TestMaterializeResolvesEveryWinner(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "password: secret://x\nplain: keep\n")

	factory := &recordingFactory{
		resolver: fakeResolver{values: map[string]string{"secret://x": "pw"}},
	}
	manager := managerFor(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		ResolverFactory: factory,
	})

	env, err := manager.Materialize("development")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if v, _ := env.Get("PASSWORD"); v != "pw" {
		t.Errorf("PASSWORD = %q, want pw", v)
	}
	if v, _ := env.Get("PLAIN"); v != "keep" {
		t.Errorf("PLAIN = %q, want keep", v)
	}

	all := env.All()
	all["PASSWORD"] = "mutated"
	if v, _ := env.Get("PASSWORD"); v != "pw" {
		t.Errorf("All did not return a copy: PASSWORD = %q", v)
	}
}

// TestMaterializeOpensOneRevealingResolver verifies Materialize opens exactly one
// resolver and always requests reveal, since a child process needs plaintext.
func TestMaterializeOpensOneRevealingResolver(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "plain: keep\n")

	factory := &recordingFactory{resolver: fakeResolver{}}
	manager := managerFor(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		ResolverFactory: factory,
	})

	if _, err := manager.Materialize("development"); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if factory.calls != 1 {
		t.Errorf("opened %d resolver(s), want exactly 1", factory.calls)
	}
	if !factory.reveal {
		t.Error("Materialize requested a masking resolver, want revealing")
	}
}

// TestMaterializeAggregatesFailures verifies a resolution failure yields a nil
// environment and an error naming every failing key.
func TestMaterializeAggregatesFailures(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "alpha: secret://a\nbeta: secret://b\n")

	factory := &recordingFactory{resolver: fakeResolver{failAll: true}}
	manager := managerFor(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		ResolverFactory: factory,
	})

	env, err := manager.Materialize("development")
	if env != nil {
		t.Error("Materialize returned a partial environment on failure")
	}
	if err == nil {
		t.Fatal("expected an aggregate resolution error")
	}
	for _, key := range []string{"ALPHA", "BETA"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error = %v, want it to name failing key %s", err, key)
		}
	}
}

// TestMaterializeObservesFileEdits verifies each operation reloads namespace files
// and opens a fresh resolver, so a later call observes edits and reuses no cached
// state.
func TestMaterializeObservesFileEdits(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "host: localhost\nsecret: secret://x\n")

	factory := &mutableFactory{value: "first"}
	manager := managerFor(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		ResolverFactory: factory,
	})

	first, err := manager.Materialize("development")
	if err != nil {
		t.Fatalf("Materialize (first): %v", err)
	}
	if v, _ := first.Get("SECRET"); v != "first" {
		t.Errorf("SECRET = %q, want first", v)
	}

	// Edit the namespace file and the resolver's output between operations.
	writeYAML(t, dir, "app.yaml", "host: edited\nsecret: secret://x\n")
	factory.value = "second"

	second, err := manager.Materialize("development")
	if err != nil {
		t.Fatalf("Materialize (second): %v", err)
	}
	if v, _ := second.Get("HOST"); v != "edited" {
		t.Errorf("HOST = %q, want edited (namespace file reloaded)", v)
	}
	if v, _ := second.Get("SECRET"); v != "second" {
		t.Errorf("SECRET = %q, want second (fresh resolver)", v)
	}
	if factory.calls != 2 {
		t.Errorf("opened %d resolver(s), want one per operation", factory.calls)
	}
}

// TestMaterializeSkipsShadowedReferences verifies a reference discarded by overlay
// precedence never reaches the resolver: a failing stale reference is replaced by
// the overlay winner, so materialization succeeds.
func TestMaterializeSkipsShadowedReferences(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "password: secret://stale\n")
	writeYAML(t, dir, "app.production.yaml", "password: replacement\n")

	factory := &recordingFactory{resolver: fakeResolver{fail: "secret://stale"}}
	manager := managerFor(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		ResolverFactory: factory,
	})

	env, err := manager.Materialize("production")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if v, _ := env.Get("PASSWORD"); v != "replacement" {
		t.Errorf("PASSWORD = %q, want replacement", v)
	}
}
