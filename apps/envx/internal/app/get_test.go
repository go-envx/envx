package app

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/file"
	"github.com/go-envx/envx/apps/envx/internal/manifest"
)

// -------------------------------------------------------------------------------------
// TestGetExistingKey verifies that Get returns the correct value for a key
// present in the merged environment.
func TestGetExistingKey(t *testing.T) {
	t.Parallel()

	dir := setupTestManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := manifest.RawFlags{Environment: "development"}
	changed := &mockFlagSet{changed: map[string]bool{"env": true}}

	val, err := app.Get(PipelineInput{
		ConfigPath: configPath,
		ProjectRef: "api-core",
		Flags:      &flags,
		Changed:    changed,
	}, "HOST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val != "localhost" {
		t.Errorf("got %q, want %q", val, "localhost")
	}
}

// -------------------------------------------------------------------------------------
// TestGetCaseInsensitive verifies that Get matches keys case-insensitively.
func TestGetCaseInsensitive(t *testing.T) {
	t.Parallel()

	dir := setupTestManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := manifest.RawFlags{Environment: "development"}
	changed := &mockFlagSet{changed: map[string]bool{"env": true}}

	val, err := app.Get(PipelineInput{
		ConfigPath: configPath,
		ProjectRef: "api-core",
		Flags:      &flags,
		Changed:    changed,
	}, "host")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val != "localhost" {
		t.Errorf("got %q, want %q", val, "localhost")
	}
}

// -------------------------------------------------------------------------------------
// TestGetNestedKey verifies that Get returns flattened nested key values.
func TestGetNestedKey(t *testing.T) {
	t.Parallel()

	dir := setupTestManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	// Add a development overlay with nested credentials.
	pgDir := filepath.Join(dir, "env")
	file.Write(t, pgDir, "postgres.development.yaml",
		"credentials:\n  password: dev-secret")

	app := New()
	flags := manifest.RawFlags{Environment: "development"}
	changed := &mockFlagSet{changed: map[string]bool{"env": true}}

	val, err := app.Get(PipelineInput{
		ConfigPath: configPath,
		ProjectRef: "api-core",
		Flags:      &flags,
		Changed:    changed,
	}, "CREDENTIALS_PASSWORD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val != "dev-secret" {
		t.Errorf("got %q, want %q", val, "dev-secret")
	}
}

// -------------------------------------------------------------------------------------
// TestGetNonexistentKey verifies that Get returns an error for a key not in
// the merged environment.
func TestGetNonexistentKey(t *testing.T) {
	t.Parallel()

	dir := setupTestManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := manifest.RawFlags{Environment: "development"}
	changed := &mockFlagSet{changed: map[string]bool{"env": true}}

	_, err := app.Get(PipelineInput{
		ConfigPath: configPath,
		ProjectRef: "api-core",
		Flags:      &flags,
		Changed:    changed,
	}, "NONEXISTENT_KEY")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}

// -------------------------------------------------------------------------------------
// TestGetDefaultEnvironment verifies that Get uses the default environment
// when no explicit environment flag is set.
func TestGetDefaultEnvironment(t *testing.T) {
	t.Parallel()

	dir := setupTestManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := manifest.RawFlags{}
	changed := &mockFlagSet{changed: map[string]bool{}}

	// With no env flag, defaults to "development".
	val, err := app.Get(PipelineInput{
		ConfigPath: configPath,
		ProjectRef: "api-core",
		Flags:      &flags,
		Changed:    changed,
	}, "HOST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val != "localhost" {
		t.Errorf("got %q, want %q", val, "localhost")
	}
}

// -------------------------------------------------------------------------------------
// TestGetInvalidProject verifies that Get returns an error for a nonexistent
// project.
func TestGetInvalidProject(t *testing.T) {
	t.Parallel()

	dir := setupTestManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := manifest.RawFlags{Environment: "development"}
	changed := &mockFlagSet{changed: map[string]bool{"env": true}}

	_, err := app.Get(PipelineInput{
		ConfigPath: configPath,
		ProjectRef: "nonexistent",
		Flags:      &flags,
		Changed:    changed,
	}, "HOST")
	if err == nil {
		t.Fatal("expected error for invalid project")
	}
}
