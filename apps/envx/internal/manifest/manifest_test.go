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
		    path: apps/api-core/env
		    includes:
		      - env/postgres
		  web:
		    path: apps/web/env
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
	if match.Project.Path != "apps/api-core/env" {
		t.Errorf("path = %q, want %q", match.Project.Path, "apps/api-core/env")
	}
	if len(match.Project.Includes) != 1 {
		t.Errorf("includes = %d, want 1", len(match.Project.Includes))
	}
}

// -------------------------------------------------------------------------------------
// TestLookupProjectByPath verifies that LookupProject falls back to matching
// by the project's path field.
func TestLookupProjectByPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "envx.yaml", `
		environments: [dev]
		projects:
		  myapp:
		    path: apps/myapp/env
	`)

	m, err := Load(filepath.Join(dir, "envx.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	match, ok := m.LookupProject("apps/myapp/env")
	if !ok {
		t.Fatal("LookupProject by path not found")
	}
	if match.Name != "myapp" {
		t.Errorf("name = %q, want %q", match.Name, "myapp")
	}
}

// -------------------------------------------------------------------------------------
// TestLookupProjectNotFound verifies that LookupProject returns false for
// a nonexistent project name or path.
func TestLookupProjectNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file.Write(t, dir, "envx.yaml", `
		environments: [dev]
		projects:
		  app:
		    path: apps/app/env
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
		    path: apps/app/env
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
// TestProjectDirRelative verifies that ProjectDir joins relative paths
// with the manifest directory.
func TestProjectDirRelative(t *testing.T) {
	t.Parallel()

	m := &Manifest{dir: "/workspace"}
	p := Project{Path: "apps/api/env"}
	got := m.ProjectDir(&p)
	want := "/workspace/apps/api/env"
	if got != want {
		t.Errorf("ProjectDir() = %q, want %q", got, want)
	}
}

// -------------------------------------------------------------------------------------
// TestProjectDirAbsolute verifies that ProjectDir returns absolute paths
// as-is without joining.
func TestProjectDirAbsolute(t *testing.T) {
	t.Parallel()

	m := &Manifest{dir: "/workspace"}
	p := Project{Path: "/absolute/path/env"}
	got := m.ProjectDir(&p)
	want := "/absolute/path/env"
	if got != want {
		t.Errorf("ProjectDir() = %q, want %q", got, want)
	}
}
