package envmerge

import (
	"path/filepath"
	"strings"
	"testing"
)

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

	env := materializeEnv(t, manager, "development")
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

	if _, err := manager.Materialize(
		MaterializeParams{Environment: "development"},
	); err != nil {
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

	result, err := manager.Materialize(MaterializeParams{Environment: "development"})
	if result != nil {
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

	first := materializeEnv(t, manager, "development")
	if v, _ := first.Get("SECRET"); v != "first" {
		t.Errorf("SECRET = %q, want first", v)
	}

	// Edit the namespace file and the resolver's output between operations.
	writeYAML(t, dir, "app.yaml", "host: edited\nsecret: secret://x\n")
	factory.value = "second"

	second := materializeEnv(t, manager, "development")
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

	env := materializeEnv(t, manager, "production")
	if v, _ := env.Get("PASSWORD"); v != "replacement" {
		t.Errorf("PASSWORD = %q, want replacement", v)
	}
}

// TestMaterializeRedactsResolvedListItemErrors verifies a list-item render failure
// identifies the item's location without exposing its resolved plaintext.
func TestMaterializeRedactsResolvedListItemErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "tokens:\n  - secret://sensitive\n")

	factory := &recordingFactory{resolver: fakeResolver{values: map[string]string{
		"secret://sensitive": "plaintext,secret",
	}}}
	manager := managerFor(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		ResolverFactory: factory,
	})

	result, err := manager.Materialize(MaterializeParams{Environment: "development"})
	if result != nil {
		t.Error("Materialize returned an environment despite a render failure")
	}
	if err == nil {
		t.Fatal("expected a delimiter render error")
	}
	if strings.Contains(err.Error(), "plaintext") {
		t.Errorf("error exposes resolved value: %v", err)
	}
	if !strings.Contains(err.Error(), `list item 1 at "tokens"`) {
		t.Errorf("error does not identify the list item: %v", err)
	}
}

// TestMaterializeJoinsListWithDefaultDelimiter verifies the default comma
// delimiter joins a list leaf when none is configured.
func TestMaterializeJoinsListWithDefaultDelimiter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "hosts:\n  - a\n  - b\n")

	res, err := mergeEnv(t, Params{
		Includes:     []string{filepath.Join(dir, "app")},
		Environments: []string{"development"},
	})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if v, _ := res.Get("HOSTS"); v != "a,b" {
		t.Errorf("HOSTS = %q, want a,b (default comma)", v)
	}
}

// TestMaterializeResolvesListReferences verifies references inside a list are
// dereferenced per item after winner selection and before the list is joined.
func TestMaterializeResolvesListReferences(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "app.yaml", "tokens:\n  - secret://a\n  - secret://b\n")

	factory := &recordingFactory{resolver: fakeResolver{values: map[string]string{
		"secret://a": "tok-a",
		"secret://b": "tok-b",
	}}}
	res, err := mergeEnv(t, Params{
		Includes:        []string{filepath.Join(dir, "app")},
		Environments:    []string{"development"},
		ResolverFactory: factory,
	})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if v, _ := res.Get("TOKENS"); v != "tok-a,tok-b" {
		t.Errorf("TOKENS = %q, want tok-a,tok-b (list items resolved)", v)
	}
}
