package envmerge

import (
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------
// setupWorkspace creates a temp workspace with one namespace (env/postgres) and
// returns the workspace dir. envmerge reads only the namespace overlays, never
// the manifest, so no envx.yaml is needed.
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

// -------------------------------------------------------------------------------------
// baseConfig builds an envmerge.Config for the temp workspace declaring the
// development and production environments.
func baseConfig(dir string) *Config {
	return &Config{
		Dir:          dir,
		Includes:     []string{"env/postgres"},
		Environments: []string{"development", "production"},
	}
}

// -------------------------------------------------------------------------------------
// TestResolveSuccess verifies a config resolves to its merged environment.
func TestResolveSuccess(t *testing.T) {
	t.Parallel()

	c := baseConfig(setupWorkspace(t))
	c.Settings = Settings{Env: "development"}
	res, err := Build(c)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("HOST = %q, want localhost (development overlay absent)", v)
	}
}

// -------------------------------------------------------------------------------------
// TestResolveDefaultEnv verifies an empty Settings.Env falls back to the first
// declared environment.
func TestResolveDefaultEnv(t *testing.T) {
	t.Parallel()

	c := baseConfig(setupWorkspace(t))
	res, err := Build(c)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := c.Environments[0]
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("HOST = %q, want localhost (default env %q)", v, want)
	}
}

// -------------------------------------------------------------------------------------
// TestResolveOverride verifies Settings.Env selects that environment's overlay
// (as diff relies on, passing each side).
func TestResolveOverride(t *testing.T) {
	t.Parallel()

	c := baseConfig(setupWorkspace(t))
	c.Settings = Settings{Env: "production"}
	res, err := Build(c)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "prod-db" {
		t.Errorf("HOST = %q, want prod-db", v)
	}
}

// -------------------------------------------------------------------------------------
// TestResolveErrors verifies an undeclared environment and a nil config fail.
func TestResolveErrors(t *testing.T) {
	t.Parallel()

	c := baseConfig(setupWorkspace(t))
	c.Settings = Settings{Env: "nope"}
	if _, err := Build(c); err == nil {
		t.Error("expected error for undeclared environment")
	}

	if _, err := Build(nil); err == nil {
		t.Error("expected error for nil config")
	}
}

// -------------------------------------------------------------------------------------
// TestDeepMerge verifies recursive map merging with scalar/list replacement.
func TestDeepMerge(t *testing.T) {
	t.Parallel()

	dst := map[string]any{
		"a": "1",
		"nested": map[string]any{
			"x": "base",
			"y": "keep",
		},
	}
	src := map[string]any{
		"b": "2",
		"nested": map[string]any{
			"x": "override",
		},
	}

	got := deepMerge(dst, src)
	if got["a"] != "1" || got["b"] != "2" {
		t.Errorf("top-level keys wrong: %v", got)
	}
	nested, _ := toMap(got["nested"])
	if nested["x"] != "override" {
		t.Errorf("nested.x = %v, want override", nested["x"])
	}
	if nested["y"] != "keep" {
		t.Errorf("nested.y = %v, want keep", nested["y"])
	}
}

// -------------------------------------------------------------------------------------
// TestFlatten verifies nested-to-env flattening and key normalization.
func TestFlatten(t *testing.T) {
	t.Parallel()

	in := map[string]any{
		"host": "localhost",
		"credentials": map[string]any{
			"user-name": "postgres",
		},
	}
	got, err := flatten(in)
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	if got["HOST"] != "localhost" {
		t.Errorf("HOST = %q, want localhost", got["HOST"])
	}
	if got["CREDENTIALS_USER_NAME"] != "postgres" {
		t.Errorf("CREDENTIALS_USER_NAME = %q, want postgres", got["CREDENTIALS_USER_NAME"])
	}
}

// -------------------------------------------------------------------------------------
// TestFlattenCollision verifies two paths collapsing to the same key error out.
func TestFlattenCollision(t *testing.T) {
	t.Parallel()

	in := map[string]any{
		"api_key": "a",
		"api": map[string]any{
			"key": "b",
		},
	}
	if _, err := flatten(in); err == nil {
		t.Fatal("expected flatten collision error")
	}
}

// -------------------------------------------------------------------------------------
// TestToEnvKey verifies dotted/hyphenated paths normalize to upper-snake.
func TestToEnvKey(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"postgres.user-name": "POSTGRES_USER_NAME",
		"host":               "HOST",
		"a.b.c":              "A_B_C",
	}
	for in, want := range cases {
		if got := toEnvKey(in); got != want {
			t.Errorf("toEnvKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// -------------------------------------------------------------------------------------
// writeYAML writes a YAML file into dir.
func writeYAML(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// -------------------------------------------------------------------------------------
// TestMergeNamespacesOverlay verifies an overlay overrides the base and origin
// tracking attributes the value to the overlay file.
func TestMergeNamespacesOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\nport: 5432\n")
	writeYAML(t, dir, "postgres.development.yaml", "host: dev-db.local\n")

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "postgres"}}, "development", mergeOptions{},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}

	if v, _ := res.Get("HOST"); v != "dev-db.local" {
		t.Errorf("HOST = %q, want dev-db.local", v)
	}
	if v, _ := res.Get("PORT"); v != "5432" {
		t.Errorf("PORT = %q, want 5432", v)
	}
	origin, ok := res.Origin("HOST")
	if !ok {
		t.Fatal("expected origin for HOST")
	}
	if filepath.Base(origin.Winner.File) != "postgres.development.yaml" {
		t.Errorf("HOST winner = %q, want overlay", origin.Winner.File)
	}
}

// -------------------------------------------------------------------------------------
// TestMergeNamespacesStrictMissingOverlay verifies strict mode errors when an
// overlay file is absent, while lax mode tolerates it.
func TestMergeNamespacesStrictMissingOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\n")

	ns := []namespace{{dir: dir, name: "postgres"}}
	_, err := mergeNamespaces(ns, "production", mergeOptions{strict: true})
	if err == nil {
		t.Error("expected strict error for missing overlay")
	}
	if _, err := mergeNamespaces(ns, "production", mergeOptions{}); err != nil {
		t.Errorf("lax mode should tolerate missing overlay, got %v", err)
	}
}

// -------------------------------------------------------------------------------------
// TestMergeNamespacesPrefixSuffix verifies global prefix/suffix and namespace
// prefixing apply to every key.
func TestMergeNamespacesPrefixSuffix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeYAML(t, dir, "postgres.yaml", "host: localhost\n")

	res, err := mergeNamespaces(
		[]namespace{{dir: dir, name: "postgres"}}, "development",
		mergeOptions{prefix: "app", suffix: "v2", namespacePrefix: true},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}
	if _, ok := res.Get("APP_POSTGRES_HOST_V2"); !ok {
		t.Errorf("expected APP_POSTGRES_HOST_V2, got keys %v", res.Keys())
	}
}
