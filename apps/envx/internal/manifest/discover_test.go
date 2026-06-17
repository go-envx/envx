package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/file"
)

// -------------------------------------------------------------------------------------
// TestDiscoverExplicitPath verifies that an explicit --config path is
// returned directly when the file exists.
func TestDiscoverExplicitPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	file.Write(t, dir, "custom.yaml", `
		environments: [dev]
		projects:
		  a:
		    includes:
		      - x/x
	`)

	got, err := Discover(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != path {
		t.Errorf("Discover() = %q, want %q", got, path)
	}
}

// -------------------------------------------------------------------------------------
// TestDiscoverExplicitPathNotFound verifies that a missing explicit path
// returns an error.
func TestDiscoverExplicitPathNotFound(t *testing.T) {
	t.Parallel()

	_, err := Discover("/nonexistent/custom.yaml")
	if err == nil {
		t.Fatal("expected error for missing explicit path")
	}
}

// -------------------------------------------------------------------------------------
// TestDiscoverEnvVar verifies that the ENVX_CONFIG environment variable
// is used when no explicit path is provided.
func TestDiscoverEnvVar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "envx.yaml")
	file.Write(t, dir, "envx.yaml", `
		environments: [dev]
		projects:
		  a:
		    includes:
		      - x/x
	`)

	t.Setenv("ENVX_CONFIG", path)

	got, err := Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != path {
		t.Errorf("Discover() = %q, want %q", got, path)
	}
}

// -------------------------------------------------------------------------------------
// TestDiscoverEnvVarNotFound verifies that an ENVX_CONFIG pointing to a
// missing file returns an error.
func TestDiscoverEnvVarNotFound(t *testing.T) {
	t.Setenv("ENVX_CONFIG", "/nonexistent/envx.yaml")

	_, err := Discover("")
	if err == nil {
		t.Fatal("expected error for missing ENVX_CONFIG path")
	}
}

// -------------------------------------------------------------------------------------
// TestDiscoverWalkUp verifies that Discover walks up parent directories
// to find envx.yaml when no explicit path or env var is set.
func TestDiscoverWalkUp(t *testing.T) {
	// Cannot be parallel because it changes the working directory.
	dir := t.TempDir()
	file.Write(t, dir, "envx.yaml", `
		environments: [dev]
		projects:
		  a:
		    includes:
		      - x/x
	`)

	// Create a nested dir and chdir into it.
	nested := filepath.Join(dir, "apps", "myapp")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}

	// Unset ENVX_CONFIG to avoid interference.
	t.Setenv("ENVX_CONFIG", "")

	got, err := Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "envx.yaml")
	if got != want {
		t.Errorf("Discover() = %q, want %q", got, want)
	}
}

// -------------------------------------------------------------------------------------
// TestDiscoverWalkUpStopsAtGitRoot verifies that the walk-up search stops
// at the .git root boundary and does not find manifests above it.
func TestDiscoverWalkUpStopsAtGitRoot(t *testing.T) {
	// Cannot be parallel because it changes the working directory.
	dir := t.TempDir()

	// Place envx.yaml ABOVE the git root — it should NOT be found.
	file.Write(t, dir, "envx.yaml", `
		environments: [dev]
		projects:
		  a:
		    includes:
		      - x/x
	`)

	// Create a nested "repo" with .git marker.
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o750); err != nil {
		t.Fatal(err)
	}

	// Create a nested dir inside the "repo".
	nested := filepath.Join(repoDir, "apps", "myapp")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ENVX_CONFIG", "")

	_, err = Discover("")
	if err == nil {
		t.Fatal("expected error: " +
			"walkUp should stop at .git root and not find envx.yaml above it",
		)
	}
}

// -------------------------------------------------------------------------------------
// TestDiscoverWalkUpNoManifest verifies that Discover returns an error when
// no envx.yaml exists anywhere in the directory tree.
func TestDiscoverWalkUpNoManifest(t *testing.T) {
	// Cannot be parallel because it changes the working directory.
	dir := t.TempDir()

	// Create a .git root with no envx.yaml.
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o750); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ENVX_CONFIG", "")

	_, err = Discover("")
	if err == nil {
		t.Fatal("expected error when no manifest exists")
	}
}
