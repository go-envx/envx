package filestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/features/workspace"
	"github.com/go-envx/envx/app/internal/features/workspace/filestore"
	"github.com/go-envx/envx/app/test/fixtures"
)

// writeManifest writes a manifest file into a fresh temp dir and returns its path.
func writeManifest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "envx.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// newRepository constructs a filestore Repository for path using the
// conventional filename.
func newRepository(t *testing.T, path string) *filestore.Repository {
	t.Helper()
	repo, err := filestore.New(filestore.Params{
		Path: path,
	})
	if err != nil {
		t.Fatalf("filestore.New: %v", err)
	}
	return repo
}

// TestNewDefaultsFilename verifies construction defaults to the conventional
// filename when omitted.
func TestNewDefaultsFilename(t *testing.T) {
	t.Parallel()

	repo, err := filestore.New(filestore.Params{})
	if err != nil {
		t.Fatalf("filestore.New: %v", err)
	}
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
}

// TestNewCustomFilename verifies construction respects a custom manifest filename.
func TestNewCustomFilename(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	customFile := "custom.yaml"
	manifestPath := filepath.Join(dir, customFile)
	content := "environments: [dev]\nprojects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(manifestPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	repo, err := filestore.New(filestore.Params{
		Path:     dir,
		Filename: customFile,
	})
	if err != nil {
		t.Fatalf("filestore.New: %v", err)
	}
	ws, err := repo.Load()
	if err != nil {
		t.Fatalf("repo.Load: %v", err)
	}
	if len(ws.Environments) != 1 || ws.Environments[0] != "dev" {
		t.Errorf("got environments %v, want [dev]", ws.Environments)
	}
}

// TestLoadValid verifies a well-formed manifest parses with its path recorded.
func TestLoadValid(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("manifest/valid-secrets")
	ws, err := newRepository(t, path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ws.Path != path {
		t.Errorf("Path = %q, want %q", ws.Path, path)
	}
	if ws.Root != filepath.Dir(path) {
		t.Errorf("Root = %q, want %q", ws.Root, filepath.Dir(path))
	}
	if !ws.HasEnvironment("production") {
		t.Error("expected production environment to be present")
	}
	if ws.Secrets.SecretsPath != "./private/secrets.yaml" {
		t.Errorf("SecretsPath = %q, want ./private/secrets.yaml", ws.Secrets.SecretsPath)
	}
	if ws.Secrets.KeysPath != "./private/envx.keys" {
		t.Errorf("KeysPath = %q, want ./private/envx.keys", ws.Secrets.KeysPath)
	}
	if ws.Secrets.Cipher != "age" {
		t.Errorf("Cipher = %q, want age", ws.Secrets.Cipher)
	}
	if _, ok := ws.LookupProject("api"); !ok {
		t.Error("expected project api to be present")
	}
}

// TestLoadFromDirectoryPath verifies a directory manifest path resolves to the
// conventional manifest inside it.
func TestLoadFromDirectoryPath(t *testing.T) {
	t.Parallel()

	manifestPath := writeManifest(t,
		"environments: [production]\nprojects:\n  app:\n    includes: [env/app]\n")
	dir := filepath.Dir(manifestPath)
	ws, err := newRepository(t, dir).Load()
	if err != nil {
		t.Fatalf("Load from directory: %v", err)
	}
	if ws.Path != manifestPath {
		t.Errorf("Path = %q, want %q", ws.Path, manifestPath)
	}
}

// TestLoadFromDirectoryMissingManifest verifies a directory manifest path with no
// manifest inside reports an actionable error wrapping ErrNotFound.
func TestLoadFromDirectoryMissingManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := newRepository(t, dir).Load()
	if err == nil {
		t.Fatal("expected an error for a directory without a manifest")
	}
	if !errors.Is(err, workspace.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
	got := err.Error()
	if !strings.Contains(got, "envx.yaml") || !strings.Contains(got, dir) {
		t.Errorf("error %q should name the manifest filename and directory", got)
	}
}

// TestLoadMissingManifest verifies an absent manifest returns ErrNotFound.
func TestLoadMissingManifest(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.yaml")
	_, err := newRepository(t, path).Load()
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
	if !errors.Is(err, workspace.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestLoadDiscovers verifies Load discovers an explicit path and loads it.
func TestLoadDiscovers(t *testing.T) {
	t.Parallel()

	ws, err := newRepository(t, fixtures.Manifest("basic")).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := ws.LookupProject("api-core"); !ok {
		t.Error("expected project api-core to be present")
	}
}
