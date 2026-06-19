package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/config"
)

// -------------------------------------------------------------------------------------
// setupWorkspace builds a temp workspace with a manifest and one namespace and
// returns the loaded manifest.
func setupWorkspace(t *testing.T, manifest string) *config.Config {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, "envx.yaml"), []byte(manifest), 0o600,
	); err != nil {
		t.Fatal(err)
	}
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

	cfg, err := config.Load(filepath.Join(dir, "envx.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

const baseManifest = `
environments: [development, production]
projects:
  db:
    includes:
      - env/postgres
`

// -------------------------------------------------------------------------------------
// TestResolveEnvSuccess verifies a project resolves to its merged environment.
func TestResolveEnvSuccess(t *testing.T) {
	t.Parallel()

	g := setupWorkspace(t, baseManifest)
	res, err := ResolveEnv(&Request{
		Config:   g,
		Project:  "db",
		Settings: Settings{Env: "development"},
	})
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("HOST = %q, want localhost (development overlay absent)", v)
	}
}

// -------------------------------------------------------------------------------------
// TestResolveEnvOverride verifies the resolved Settings.Env selects that
// environment's overlay (as diff relies on, passing each side).
func TestResolveEnvOverride(t *testing.T) {
	t.Parallel()

	g := setupWorkspace(t, baseManifest)
	res, err := ResolveEnv(&Request{
		Config:   g,
		Project:  "db",
		Settings: Settings{Env: "production"},
	})
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "prod-db" {
		t.Errorf("HOST = %q, want prod-db", v)
	}
}

// -------------------------------------------------------------------------------------
// TestResolveEnvErrors verifies unknown projects and undeclared environments
// fail.
func TestResolveEnvErrors(t *testing.T) {
	t.Parallel()

	g := setupWorkspace(t, baseManifest)

	if _, err := ResolveEnv(&Request{
		Config: g, Project: "missing",
		Settings: Settings{Env: "development"},
	}); err == nil {
		t.Error("expected error for unknown project")
	}

	if _, err := ResolveEnv(&Request{
		Config: g, Project: "db", Settings: Settings{Env: "nope"},
	}); err == nil {
		t.Error("expected error for undeclared environment")
	}
}
