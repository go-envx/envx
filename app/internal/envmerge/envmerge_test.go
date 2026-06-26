package envmerge

import (
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

// baseParams builds envmerge.Params for the temp workspace declaring the
// development and production environments.
func baseParams(dir string) Params {
	return Params{
		Includes:     []string{filepath.Join(dir, "env", "postgres")},
		Environments: []string{"development", "production"},
	}
}

// -------------------------------------------------------------------------------------

// TestResolveSuccess verifies params resolve to the merged environment.
func TestResolveSuccess(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	p.Settings = Settings{Env: "development"}
	res, err := Build(p)
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

	p := baseParams(setupWorkspace(t))
	res, err := Build(p)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := p.Environments[0]
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("HOST = %q, want localhost (default env %q)", v, want)
	}
}

// -------------------------------------------------------------------------------------

// TestResolveOverride verifies Settings.Env selects that environment's overlay
// (as diff relies on, passing each side).
func TestResolveOverride(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	p.Settings = Settings{Env: "production"}
	res, err := Build(p)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "prod-db" {
		t.Errorf("HOST = %q, want prod-db", v)
	}
}

// -------------------------------------------------------------------------------------

// TestResolveErrors verifies an undeclared environment fails.
func TestResolveErrors(t *testing.T) {
	t.Parallel()

	p := baseParams(setupWorkspace(t))
	p.Settings = Settings{Env: "nope"}
	if _, err := Build(p); err == nil {
		t.Error("expected error for undeclared environment")
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
		[]namespace{{dir: dir, name: "postgres"}}, Settings{Env: "development"},
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
	_, err := mergeNamespaces(ns, Settings{Env: "production", Strict: true})
	if err == nil {
		t.Error("expected strict error for missing overlay")
	}
	if _, err := mergeNamespaces(ns, Settings{Env: "production"}); err != nil {
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
		[]namespace{{dir: dir, name: "postgres"}},
		Settings{Env: "development", Prefix: "app", Suffix: "v2", NamespacePrefix: true},
	)
	if err != nil {
		t.Fatalf("mergeNamespaces: %v", err)
	}
	if _, ok := res.Get("APP_POSTGRES_HOST_V2"); !ok {
		t.Errorf("expected APP_POSTGRES_HOST_V2, got keys %v", res.Keys())
	}
}
