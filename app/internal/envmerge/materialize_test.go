package envmerge

import (
	"os"
	"path/filepath"
	"strings"
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

// TestResolveSuccess verifies params resolve to the merged environment.
func TestResolveSuccess(t *testing.T) {
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

// TestResolveDefaultEnv verifies an empty DefaultEnvironment falls back to the
// first declared environment.
func TestResolveDefaultEnv(t *testing.T) {
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

// TestResolveOverride verifies DefaultEnvironment selects that environment's
// overlay (as diff relies on, passing each side).
func TestResolveOverride(t *testing.T) {
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

// TestResolveErrors verifies an undeclared environment fails.
func TestResolveErrors(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	p.DefaultEnvironment = "nope"
	if _, err := mergeEnv(t, p); err == nil {
		t.Error("expected error for undeclared environment")
	}
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

	env, err := manager.Materialize("development")
	if env != nil {
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
