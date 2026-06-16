package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/file"
	"github.com/go-envx/envx/apps/envx/internal/runner"
)

// -------------------------------------------------------------------------------------
// mockFlagSet implements config.FlagSet for testing.
type mockFlagSet struct {
	changed map[string]bool
}

// -------------------------------------------------------------------------------------
// Changed reports whether the specified flag was changed. The test sets up the
// changed map to control which flags are considered changed.
func (m *mockFlagSet) Changed(name string) bool {
	return m.changed[name]
}

// -------------------------------------------------------------------------------------
// setupTestManifest creates a temporary directory with a manifest and env files
// for testing. It returns the path to the temp directory.
func setupTestManifest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create manifest.
	file.Write(t, dir, "envx.yaml", `
		environments: [development, production]
		projects:
		  api-core:
		    path: apps/api-core/env
		    includes:
		      - env/postgres
	`)

	// Create env dir with files.
	pgDir := filepath.Join(dir, "env")
	if err := os.MkdirAll(pgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	file.Write(t, pgDir, "postgres.yaml", "host: localhost")

	appDir := filepath.Join(dir, "apps", "api-core", "env")
	if err := os.MkdirAll(appDir, 0o750); err != nil {
		t.Fatal(err)
	}
	file.Write(t, appDir, "api-core.yaml", "port: 3000")

	return dir
}

// -------------------------------------------------------------------------------------
// TestResolvePipelineSuccess verifies the full pipeline produces a valid
// manifest, project match, and non-empty merged environment.
func TestResolvePipelineSuccess(t *testing.T) {
	t.Parallel()

	dir := setupTestManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := config.RawFlags{}
	changed := &mockFlagSet{changed: map[string]bool{}}

	pipeline, result, err := app.ResolvePipeline(
		configPath,
		"api-core",
		"development",
		flags,
		changed,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pipeline == nil {
		t.Fatal("expected non-nil pipeline")
	}
	if pipeline.Manifest == nil {
		t.Fatal("expected non-nil manifest in pipeline")
	}
	if pipeline.Project.Name != "api-core" {
		t.Errorf("project name = %q, want %q", pipeline.Project.Name, "api-core")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Env) == 0 {
		t.Error("expected non-empty env result")
	}
}

// -------------------------------------------------------------------------------------
// TestResolvePipelineInvalidProject verifies that a nonexistent project
// reference returns an error.
func TestResolvePipelineInvalidProject(t *testing.T) {
	t.Parallel()

	dir := setupTestManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := config.RawFlags{}
	changed := &mockFlagSet{changed: map[string]bool{}}

	_, _, err := app.ResolvePipeline(
		configPath,
		"nonexistent",
		"development",
		flags,
		changed,
	)
	if err == nil {
		t.Fatal("expected error for invalid project")
	}
}

// -------------------------------------------------------------------------------------
// TestResolvePipelineInvalidEnvironment verifies that an undeclared
// environment returns an error.
func TestResolvePipelineInvalidEnvironment(t *testing.T) {
	t.Parallel()

	dir := setupTestManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := config.RawFlags{}
	changed := &mockFlagSet{changed: map[string]bool{}}

	_, _, err := app.ResolvePipeline(
		configPath,
		"api-core",
		"nonexistent",
		flags,
		changed,
	)
	if err == nil {
		t.Fatal("expected error for invalid environment")
	}
}

// -------------------------------------------------------------------------------------
// TestResolvePipelineBadConfigPath verifies that a missing manifest file
// returns an error.
func TestResolvePipelineBadConfigPath(t *testing.T) {
	t.Parallel()

	app := New()
	flags := config.RawFlags{}
	changed := &mockFlagSet{changed: map[string]bool{}}

	_, _, err := app.ResolvePipeline(
		"/nonexistent/path/envx.yaml",
		"api-core",
		"development",
		flags,
		changed,
	)
	if err == nil {
		t.Fatal("expected error for bad config path")
	}
}

// -------------------------------------------------------------------------------------
// TestRunWithMockRunner verifies that Run calls the injected runner with
// the merged environment.
func TestRunWithMockRunner(t *testing.T) {
	t.Parallel()

	dir := setupTestManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	var capturedEnv map[string]string
	app := &App{
		ConfigResolver: config.NewResolver(),
		Runner: func(_ context.Context, args []string, opts runner.Options) error {
			capturedEnv = opts.Env
			return nil
		},
	}

	flags := config.RawFlags{}
	changed := &mockFlagSet{changed: map[string]bool{}}

	err := app.Run(
		context.Background(),
		configPath,
		"api-core",
		"development",
		flags,
		changed,
		RunOptions{
			Args: []string{"echo", "test"},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedEnv == nil {
		t.Fatal("expected runner to be called with env")
	}
}
