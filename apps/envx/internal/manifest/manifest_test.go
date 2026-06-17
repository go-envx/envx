package manifest

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/file"
)

// -------------------------------------------------------------------------------------
// TestLookupProjectByName verifies that LookupProject finds a project by
// its exact name key in the projects map.
func TestLookupProjectByName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "envx.yaml", `
		environments: [dev]
		projects:
		  api-core:
		    includes:
		      - env/postgres
		      - apps/api-core/env/api-core
		  web:
		    includes:
		      - apps/web/env/web
	`)

	m, err := Load(filepath.Join(dir, "envx.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	match, ok := m.LookupProject("api-core")
	if !ok {
		t.Fatal("LookupProject(api-core) not found")
	}
	if match.Name != "api-core" {
		t.Errorf("name = %q, want %q", match.Name, "api-core")
	}
	if len(match.Project.Includes) != 2 {
		t.Errorf("includes = %d, want 2", len(match.Project.Includes))
	}
}

// -------------------------------------------------------------------------------------
// TestLookupProjectNotFound verifies that LookupProject returns false for
// a nonexistent project name.
func TestLookupProjectNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "envx.yaml", `
		environments: [dev]
		projects:
		  app:
		    includes:
		      - apps/app/env/app
	`)

	m, err := Load(filepath.Join(dir, "envx.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := m.LookupProject("nonexistent")
	if ok {
		t.Error("expected LookupProject to return false for nonexistent project")
	}
}

// -------------------------------------------------------------------------------------
// TestHasEnvironment verifies that HasEnvironment correctly identifies
// declared and undeclared environment names.
func TestHasEnvironment(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "envx.yaml", `
		environments: [development, production]
		projects:
		  app:
		    includes:
		      - apps/app/env/app
	`)

	m, err := Load(filepath.Join(dir, "envx.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !m.HasEnvironment("development") {
		t.Error("expected development to be a valid environment")
	}
	if m.HasEnvironment("staging") {
		t.Error("expected staging to not be a valid environment")
	}
}

// -------------------------------------------------------------------------------------
// TestLookupIncludeFound verifies that LookupInclude resolves an include path
// to its absolute directory and base name.
func TestLookupIncludeFound(t *testing.T) {
	t.Parallel()

	m := &Manifest{
		dir: "/workspace",
		Projects: map[string]Project{
			"api-core": {Includes: []string{"env/postgres", "env/gateway"}},
		},
	}

	dir, name, ok := m.LookupInclude("env/postgres")
	if !ok {
		t.Fatal("LookupInclude(env/postgres) not found")
	}
	if dir != "/workspace/env" {
		t.Errorf("dir = %q, want %q", dir, "/workspace/env")
	}
	if name != "postgres" {
		t.Errorf("name = %q, want %q", name, "postgres")
	}
}

// -------------------------------------------------------------------------------------
// TestLookupIncludeNotFound verifies that LookupInclude returns false for
// an include path not present in any project.
func TestLookupIncludeNotFound(t *testing.T) {
	t.Parallel()

	m := &Manifest{
		dir: "/workspace",
		Projects: map[string]Project{
			"api-core": {Includes: []string{"env/postgres"}},
		},
	}

	_, _, ok := m.LookupInclude("env/nonexistent")
	if ok {
		t.Error("expected LookupInclude to return false for unknown include")
	}
}
