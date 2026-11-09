package filestore_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/go-envx/envx/app/internal/core/env/filestore"
	"github.com/go-envx/envx/app/internal/lib/filex"
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

// readFile returns the content of path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := filex.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return string(data)
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

// TestSetOverlay verifies creation, in-place updates that preserve comments
// and blank lines, and lock protection.
func TestSetOverlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	include := filepath.Join(dir, "app")
	var repo filestore.Repository

	path, err := repo.SetOverlay(include, "prod", "host", "prod.local")
	if err != nil {
		t.Fatalf("SetOverlay(new): %v", err)
	}
	if path != filepath.Join(dir, "app.prod.yaml") {
		t.Errorf("path = %q", path)
	}
	if got := readFile(t, path); got != "host: prod.local\n" {
		t.Errorf("content = %q", got)
	}

	writeFile(t, dir, "app.prod.yaml", "# comment\nhost: old\n\ndb:\n    port: 5432\n")
	if _, err := repo.SetOverlay(include, "prod", "host", "new"); err != nil {
		t.Fatalf("SetOverlay(update): %v", err)
	}
	if _, err := repo.SetOverlay(include, "prod", "db.name", "mydb"); err != nil {
		t.Fatalf("SetOverlay(nested): %v", err)
	}
	want := "# comment\nhost: new\n\ndb:\n    port: 5432\n    name: mydb\n"
	if got := readFile(t, path); got != want {
		t.Errorf("content = %q, want %q", got, want)
	}

	if _, err := os.Stat(filepath.Join(dir, ".envx.lock")); err != nil {
		t.Errorf("lock file: %v", err)
	}
}

// TestSetOverlayConcurrent verifies concurrent writes to one overlay are
// serialized so no update is lost.
func TestSetOverlayConcurrent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	include := filepath.Join(dir, "app")
	var repo filestore.Repository
	var wg sync.WaitGroup
	const writers = 8
	for i := range writers {
		wg.Go(func() {
			key := fmt.Sprintf("k%d", i)
			if _, err := repo.SetOverlay(include, "prod", key, "v"); err != nil {
				t.Errorf("SetOverlay(%s): %v", key, err)
			}
		})
	}
	wg.Wait()

	data, found, err := repo.LoadOverlay(include, "prod")
	if err != nil || !found || len(data.Data) != writers {
		t.Errorf("overlay = %v, %v, %v; want %d keys", data.Data, found, err, writers)
	}
}

// TestSetOverlayRefusesCollections verifies lists and mappings are never
// overwritten and the file is left unchanged.
func TestSetOverlayRefusesCollections(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	initial := "items:\n  - a\ndb:\n  port: 1\n"
	path := writeFile(t, dir, "app.prod.yaml", initial)
	include := filepath.Join(dir, "app")

	var repo filestore.Repository
	for _, key := range []string{"items", "db", "items.child"} {
		if _, err := repo.SetOverlay(include, "prod", key, "x"); err == nil {
			t.Errorf("SetOverlay(%s) succeeded", key)
		}
	}
	if got := readFile(t, path); got != initial {
		t.Errorf("content changed to %q", got)
	}
}
