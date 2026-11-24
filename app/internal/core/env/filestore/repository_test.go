package filestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/core/env/filestore"
)

// writeFile writes content to dir/name.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// TestLoadBase verifies loading with or without the extension, empty files,
// and failure modes.
func TestLoadBase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeFile(t, dir, "app.yaml", "host: localhost\nport: 5432\n")
	writeFile(t, dir, "empty.yaml", "")
	writeFile(t, dir, "bad.yaml", ":\n- invalid")

	var repo filestore.Repository
	for _, include := range []string{filepath.Join(dir, "app"), path} {
		data, err := repo.LoadBase(include)
		if err != nil {
			t.Fatalf("LoadBase(%s): %v", include, err)
		}
		if data.SourcePath != path || data.Data["host"] != "localhost" ||
			data.Data["port"] != 5432 {
			t.Errorf("LoadBase(%s) = %+v", include, data)
		}
	}

	empty, err := repo.LoadBase(filepath.Join(dir, "empty"))
	if err != nil || empty.Data == nil || len(empty.Data) != 0 {
		t.Errorf("LoadBase(empty) = %+v, %v; want empty map", empty, err)
	}
	_, err = repo.LoadBase(filepath.Join(dir, "missing"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing error = %v, want ErrNotExist", err)
	}
	if _, err := repo.LoadBase(filepath.Join(dir, "bad")); err == nil {
		t.Error("malformed base loaded")
	}
}

// TestLoadOverlay verifies found, missing, and malformed overlays.
func TestLoadOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeFile(t, dir, "app.production.yaml", "host: prod\n")
	writeFile(t, dir, "app.broken.yaml", ":\n- invalid")
	include := filepath.Join(dir, "app")

	var repo filestore.Repository
	data, found, err := repo.LoadOverlay(include, "production")
	if err != nil || !found || data.SourcePath != path || data.Data["host"] != "prod" {
		t.Errorf("LoadOverlay(production) = %+v, %v, %v", data, found, err)
	}
	data, found, err = repo.LoadOverlay(include, "staging")
	want := filepath.Join(dir, "app.staging.yaml")
	if err != nil || found || data.SourcePath != want {
		t.Errorf("LoadOverlay(staging) = %+v, %v, %v", data, found, err)
	}
	if _, found, err := repo.LoadOverlay(include, "broken"); err == nil || found {
		t.Errorf("LoadOverlay(broken) = %v, %v; want error", found, err)
	}
}
