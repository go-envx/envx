package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/config"
)

// -------------------------------------------------------------------------------------
// engineFlagSet is a test double for config.FlagSet.
type engineFlagSet struct {
	changed map[string]bool
}

// -------------------------------------------------------------------------------------
// Changed reports whether name was marked changed in the fixture.
func (f engineFlagSet) Changed(name string) bool { return f.changed[name] }

// -------------------------------------------------------------------------------------
// setupWorkspace builds a temp workspace with a manifest and one namespace and
// returns the loaded global context.
func setupWorkspace(t *testing.T, manifest string) config.Global {
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
	return config.Global{Config: cfg, Environment: "development"}
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
		Global:  g,
		Project: "db",
		Changed: engineFlagSet{changed: map[string]bool{}},
	})
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "localhost" {
		t.Errorf("HOST = %q, want localhost (development overlay absent)", v)
	}
}

// -------------------------------------------------------------------------------------
// TestResolveEnvOverride verifies an explicit Request.Environment selects that
// environment's overlay (as diff relies on).
func TestResolveEnvOverride(t *testing.T) {
	t.Parallel()

	g := setupWorkspace(t, baseManifest)
	res, err := ResolveEnv(&Request{
		Global:      g,
		Project:     "db",
		Environment: "production",
		Changed:     engineFlagSet{changed: map[string]bool{}},
	})
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "prod-db" {
		t.Errorf("HOST = %q, want prod-db", v)
	}
}

// -------------------------------------------------------------------------------------
// TestResolveEnvProjectDefault verifies a project default_environment is used
// when neither --env nor ENVX_ENV is set.
func TestResolveEnvProjectDefault(t *testing.T) {
	g := setupWorkspace(t, `
environments: [development, production]
projects:
  db:
    settings:
      default_environment: production
    includes:
      - env/postgres
`)
	// Global.Environment defaults to development; the project default should
	// win because the user did not set --env / ENVX_ENV.
	res, err := ResolveEnv(&Request{
		Global:  g,
		Project: "db",
		Changed: engineFlagSet{changed: map[string]bool{}},
	})
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}
	if v, _ := res.Get("HOST"); v != "prod-db" {
		t.Errorf("HOST = %q, want prod-db (project default)", v)
	}
}

// -------------------------------------------------------------------------------------
// TestResolveEnvErrors verifies unknown projects and undeclared environments
// fail.
func TestResolveEnvErrors(t *testing.T) {
	t.Parallel()

	g := setupWorkspace(t, baseManifest)

	if _, err := ResolveEnv(&Request{
		Global: g, Project: "missing",
		Changed: engineFlagSet{changed: map[string]bool{}},
	}); err == nil {
		t.Error("expected error for unknown project")
	}

	if _, err := ResolveEnv(&Request{
		Global: g, Project: "db", Environment: "nope",
		Changed: engineFlagSet{changed: map[string]bool{}},
	}); err == nil {
		t.Error("expected error for undeclared environment")
	}
}
