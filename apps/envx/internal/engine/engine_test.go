package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------
// setupWorkspace creates a temp workspace with one namespace (env/postgres) and
// returns the workspace dir. The engine reads only the namespace overlays, never
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
// baseConfig builds an engine.Config for the temp workspace declaring the
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
// TestResolveDefaultEnv verifies an empty Settings.Env falls back to DefaultEnv.
func TestResolveDefaultEnv(t *testing.T) {
	t.Parallel()

	c := baseConfig(setupWorkspace(t))
	res, err := Build(c)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("HOST = %q, want localhost (default env %q)", v, DefaultEnv)
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
