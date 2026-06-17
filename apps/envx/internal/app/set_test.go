package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/file"
	"github.com/go-envx/envx/apps/envx/internal/manifest"

	"gopkg.in/yaml.v3"
)

// -------------------------------------------------------------------------------------
// setupSetManifest creates a temporary directory with a manifest and namespace
// files suitable for testing the Set method.
func setupSetManifest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	file.Write(t, dir, "envx.yaml", `
		environments: [development, staging]
		projects:
		  api-core:
		    includes:
		      - env/postgres
		      - apps/api-core/env/api-core
	`)

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
// TestSetSimpleKey verifies that Set writes a simple key-value pair to the
// correct overlay file.
func TestSetSimpleKey(t *testing.T) {
	t.Parallel()

	dir := setupSetManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := manifest.RawFlags{Environment: "development"}
	changed := &mockFlagSet{changed: map[string]bool{"env": true}}

	err := app.Set(&SetInput{
		ConfigPath:  configPath,
		IncludePath: "env/postgres",
		Flags:       &flags,
		Changed:     changed,
		Key:         "host",
		Value:       "new-db.local",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	targetFile := filepath.Join(dir, "env", "postgres.development.yaml")
	got := readYAMLKey(t, targetFile, "host")
	if got != "new-db.local" {
		t.Errorf("got %q, want %q", got, "new-db.local")
	}
}

// -------------------------------------------------------------------------------------
// TestSetNestedKey verifies that Set supports dot-separated key paths for
// nested YAML structures.
func TestSetNestedKey(t *testing.T) {
	t.Parallel()

	dir := setupSetManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := manifest.RawFlags{Environment: "staging"}
	changed := &mockFlagSet{changed: map[string]bool{"env": true}}

	err := app.Set(&SetInput{
		ConfigPath:  configPath,
		IncludePath: "env/postgres",
		Flags:       &flags,
		Changed:     changed,
		Key:         "credentials.password",
		Value:       "staging-secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	targetFile := filepath.Join(dir, "env", "postgres.staging.yaml")
	got := readNestedYAMLKey(t, targetFile, "credentials.password")
	if got != "staging-secret" {
		t.Errorf("got %q, want %q", got, "staging-secret")
	}
}

// -------------------------------------------------------------------------------------
// TestSetDefaultEnvironment verifies that Set uses the default environment
// when no explicit environment flag is set.
func TestSetDefaultEnvironment(t *testing.T) {
	t.Parallel()

	dir := setupSetManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := manifest.RawFlags{}
	changed := &mockFlagSet{changed: map[string]bool{}}

	err := app.Set(&SetInput{
		ConfigPath:  configPath,
		IncludePath: "env/postgres",
		Flags:       &flags,
		Changed:     changed,
		Key:         "port",
		Value:       "5433",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Defaults to development.
	targetFile := filepath.Join(dir, "env", "postgres.development.yaml")
	got := readYAMLKey(t, targetFile, "port")
	if got != "5433" {
		t.Errorf("got %q, want %q", got, "5433")
	}
}

// -------------------------------------------------------------------------------------
// TestSetUnknownIncludePath verifies that Set returns an error when the
// include path doesn't exist in the manifest.
func TestSetUnknownIncludePath(t *testing.T) {
	t.Parallel()

	dir := setupSetManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := manifest.RawFlags{Environment: "development"}
	changed := &mockFlagSet{changed: map[string]bool{"env": true}}

	err := app.Set(&SetInput{
		ConfigPath:  configPath,
		IncludePath: "env/unknown-ns",
		Flags:       &flags,
		Changed:     changed,
		Key:         "key",
		Value:       "value",
	})
	if err == nil {
		t.Fatal("expected error for unknown include path")
	}
	if !strings.Contains(err.Error(), "not found in manifest") {
		t.Errorf("error %q does not mention not found", err.Error())
	}
}

// -------------------------------------------------------------------------------------
// TestSetInvalidEnvironment verifies that Set returns an error when the
// environment is not declared in the manifest.
func TestSetInvalidEnvironment(t *testing.T) {
	t.Parallel()

	dir := setupSetManifest(t)
	configPath := filepath.Join(dir, "envx.yaml")

	app := New()
	flags := manifest.RawFlags{Environment: "nonexistent"}
	changed := &mockFlagSet{changed: map[string]bool{"env": true}}

	err := app.Set(&SetInput{
		ConfigPath:  configPath,
		IncludePath: "env/postgres",
		Flags:       &flags,
		Changed:     changed,
		Key:         "host",
		Value:       "value",
	})
	if err == nil {
		t.Fatal("expected error for invalid environment")
	}
}

// -------------------------------------------------------------------------------------
// readYAMLKey reads a YAML file and returns the string value of a top-level key.
func readYAMLKey(t *testing.T, path, key string) string {
	t.Helper()
	//nolint:gosec // test helper
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var contents map[string]any
	if err := yaml.Unmarshal(data, &contents); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	val, ok := contents[key]
	if !ok {
		t.Fatalf("key %q not found in %s", key, path)
	}
	return fmt.Sprint(val)
}

// -------------------------------------------------------------------------------------
// readNestedYAMLKey reads a YAML file and returns the string value at a
// dot-separated key path.
func readNestedYAMLKey(t *testing.T, path, key string) string {
	t.Helper()
	//nolint:gosec // test helper
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var contents map[string]any
	if err := yaml.Unmarshal(data, &contents); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	parts := strings.Split(key, ".")
	current := any(contents)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("key %q: expected map at %q, got %T", key, part, current)
		}
		current, ok = m[part]
		if !ok {
			t.Fatalf("key %q not found at %q in %s", key, part, path)
		}
	}
	return fmt.Sprint(current)
}
