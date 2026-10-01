package filestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/features/env/filestore"
	"github.com/go-envx/envx/app/internal/utils/filex"
)

func TestNew(t *testing.T) {
	t.Parallel()

	repo := filestore.New(filestore.Params{})
	if repo == nil {
		t.Fatal("New returned nil")
	}
}

func TestLoadBaseSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	basePath := filepath.Join(dir, "app.yaml")
	content := "host: localhost\nport: 5432\n"
	if err := os.WriteFile(basePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo := filestore.New(filestore.Params{})

	// Test with path without extension
	data, err := repo.LoadBase(filepath.Join(dir, "app"))
	if err != nil {
		t.Fatalf("LoadBase without extension: %v", err)
	}
	if data.SourcePath != basePath {
		t.Errorf("SourcePath = %q, want %q", data.SourcePath, basePath)
	}
	if data.Data["host"] != "localhost" || data.Data["port"] != 5432 {
		t.Errorf("unexpected Data: %+v", data.Data)
	}

	// Test with path with .yaml extension
	dataExt, err := repo.LoadBase(basePath)
	if err != nil {
		t.Fatalf("LoadBase with extension: %v", err)
	}
	if dataExt.SourcePath != basePath {
		t.Errorf("SourcePath = %q, want %q", dataExt.SourcePath, basePath)
	}
}

func TestLoadBaseMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo := filestore.New(filestore.Params{})

	_, err := repo.LoadBase(filepath.Join(dir, "missing"))
	if err == nil {
		t.Fatal("expected error for missing base file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got: %v", err)
	}
}

func TestLoadBaseMalformedYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	basePath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(basePath, []byte(":\n- invalid"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo := filestore.New(filestore.Params{})
	_, err := repo.LoadBase(filepath.Join(dir, "bad"))
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestLoadBaseEmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	basePath := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(basePath, []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo := filestore.New(filestore.Params{})
	data, err := repo.LoadBase(filepath.Join(dir, "empty"))
	if err != nil {
		t.Fatalf("LoadBase empty file: %v", err)
	}
	if data.Data == nil || len(data.Data) != 0 {
		t.Errorf("Data = %+v, want empty map", data.Data)
	}
}

func TestLoadOverlaySuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "app.production.yaml")
	content := "host: prod-db.internal\n"
	if err := os.WriteFile(overlayPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo := filestore.New(filestore.Params{})

	data, found, err := repo.LoadOverlay(filepath.Join(dir, "app"), "production")
	if err != nil {
		t.Fatalf("LoadOverlay: %v", err)
	}
	if !found {
		t.Fatal("expected found = true")
	}
	if data.SourcePath != overlayPath {
		t.Errorf("SourcePath = %q, want %q", data.SourcePath, overlayPath)
	}
	if data.Data["host"] != "prod-db.internal" {
		t.Errorf("unexpected Data: %+v", data.Data)
	}
}

func TestLoadOverlayMissingReturnsFalse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo := filestore.New(filestore.Params{})

	data, found, err := repo.LoadOverlay(filepath.Join(dir, "app"), "staging")
	if err != nil {
		t.Fatalf("LoadOverlay unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected found = false for missing overlay")
	}
	expectedPath := filepath.Join(dir, "app.staging.yaml")
	if data.SourcePath != expectedPath {
		t.Errorf("SourcePath = %q, want %q", data.SourcePath, expectedPath)
	}
}

func TestLoadOverlayMalformedYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "app.staging.yaml")
	if err := os.WriteFile(overlayPath, []byte(":\n- invalid"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo := filestore.New(filestore.Params{})
	_, found, err := repo.LoadOverlay(filepath.Join(dir, "app"), "staging")
	if err == nil {
		t.Fatal("expected error for malformed overlay YAML")
	}
	if found {
		t.Fatal("expected found = false on error")
	}
}

func TestSetOverlayCreatesNewFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo := filestore.New(filestore.Params{})

	path, err := repo.SetOverlay(filepath.Join(dir, "app"), "prod", "host", "prod.local")
	if err != nil {
		t.Fatalf("SetOverlay: %v", err)
	}

	expectedPath := filepath.Join(dir, "app.prod.yaml")
	if path != expectedPath {
		t.Errorf("path = %q, want %q", path, expectedPath)
	}

	content, err := filex.Read(expectedPath)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(content) != "host: prod.local\n" {
		t.Errorf("content = %q, want host: prod.local\n", string(content))
	}
}

func TestSetOverlayUpdatesExistingKeyAndNested(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.prod.yaml")
	initial := "# comment\nhost: old\n\ndb:\n  port: 5432\n"
	if err := os.WriteFile(filePath, []byte(initial), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo := filestore.New(filestore.Params{})

	_, err := repo.SetOverlay(filepath.Join(dir, "app"), "prod", "host", "new")
	if err != nil {
		t.Fatalf("SetOverlay update host: %v", err)
	}

	_, err = repo.SetOverlay(filepath.Join(dir, "app"), "prod", "db.name", "mydb")
	if err != nil {
		t.Fatalf("SetOverlay nested: %v", err)
	}

	data, found, err := repo.LoadOverlay(filepath.Join(dir, "app"), "prod")
	if err != nil || !found {
		t.Fatalf("LoadOverlay failed: %v", err)
	}
	if data.Data["host"] != "new" {
		t.Errorf("host = %v, want new", data.Data["host"])
	}
	dbMap, ok := data.Data["db"].(map[string]any)
	if !ok {
		t.Fatalf("db is not map: %T", data.Data["db"])
	}
	if dbMap["name"] != "mydb" || dbMap["port"] != 5432 {
		t.Errorf("dbMap = %+v, want name: mydb, port: 5432", dbMap)
	}
}

func TestSetOverlayRefusesToOverwriteCollection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.prod.yaml")
	initial := "items:\n  - a\n  - b\n"
	if err := os.WriteFile(filePath, []byte(initial), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo := filestore.New(filestore.Params{})

	_, err := repo.SetOverlay(filepath.Join(dir, "app"), "prod", "items", "scalar")
	if err == nil {
		t.Fatal("expected error when trying to overwrite list with scalar")
	}
}
