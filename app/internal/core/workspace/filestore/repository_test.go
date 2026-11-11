package filestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/core/workspace/filestore"
	"github.com/go-envx/envx/app/internal/lib/str"
)

var minimalManifest = str.Dedent(`
	environments: [dev]
	projects:
	  app:
	    includes: [env/app]
`)

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
	if err := os.WriteFile(manifestPath, []byte(minimalManifest), 0o600); err != nil {
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

	path := writeManifest(t, str.Dedent(`
		environments: [development, production]
		secrets:
		  path: ./private/secrets.yaml
		  keys-path: ./private/envx.keys
		  cipher: age
		projects:
		  api:
		    includes:
		      - env/postgres
	`))
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
	wantSecrets := filepath.Join(ws.Root, "private", "secrets.yaml")
	if ws.Secrets.SecretsPath != wantSecrets {
		t.Errorf("SecretsPath = %q, want %q", ws.Secrets.SecretsPath, wantSecrets)
	}
	wantKeys := filepath.Join(ws.Root, "private", "envx.keys")
	if ws.Secrets.KeysPath != wantKeys {
		t.Errorf("KeysPath = %q, want %q", ws.Secrets.KeysPath, wantKeys)
	}
	if ws.Secrets.Cipher != "age" {
		t.Errorf("Cipher = %q, want age", ws.Secrets.Cipher)
	}
	if _, ok := ws.LookupProject("api"); !ok {
		t.Error("expected project api to be present")
	}
}

// TestLoadProviders verifies declared providers load with default timeouts and
// undeclared providers stay disabled.
func TestLoadProviders(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, minimalManifest+"\n"+str.Dedent(`
		secrets:
		  providers:
		    op: {}
		    passwordstate:
		      url: https://pw.example.com
		      api-key-env: PW_KEY
		      list-id: 1990
		      field: GenericField1
		      timeout: 5s
	`))
	ws, err := newRepository(t, path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := workspace.ProvidersConfig{
		OnePassword: &workspace.CLIProviderConfig{Timeout: 30 * time.Second},
		Passwordstate: &workspace.PasswordstateConfig{
			Timeout:   5 * time.Second,
			URL:       "https://pw.example.com",
			APIKeyEnv: "PW_KEY",
			ListID:    1990,
			Field:     "GenericField1",
		},
	}
	if !reflect.DeepEqual(ws.Secrets.Providers, want) {
		t.Errorf("Providers = %+v, want %+v", ws.Secrets.Providers, want)
	}
}

// TestLoadProvidersRejectsInvalid verifies unknown providers and invalid
// provider settings fail at load.
func TestLoadProvidersRejectsInvalid(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		body string
		want string
	}{
		"unknown provider": {
			body: "secrets:\n  providers:\n    vault: {}\n",
			want: `unknown secrets provider "vault"`,
		},
		"bad timeout": {
			body: "secrets:\n  providers:\n    bws:\n      timeout: soon\n",
			want: "positive duration",
		},
		"negative timeout": {
			body: "secrets:\n  providers:\n    op:\n      timeout: -1s\n",
			want: "positive duration",
		},
		"zero list-id": {
			body: "secrets:\n  providers:\n    passwordstate:\n      list-id: 0\n",
			want: `list-id "0" must be a positive integer`,
		},
		"non-numeric list-id": {
			body: "secrets:\n  providers:\n    passwordstate:\n      list-id: web\n",
			want: `list-id "web" must be a positive integer`,
		},
		"empty field": {
			body: "secrets:\n  providers:\n    passwordstate:\n      field: \"\"\n",
			want: "field must be a non-empty string",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := writeManifest(t, minimalManifest+"\n"+tt.body)
			_, err := newRepository(t, path).Load()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Load error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

// TestLoadFromDirectoryPath verifies a directory manifest path resolves to the
// conventional manifest inside it.
func TestLoadFromDirectoryPath(t *testing.T) {
	t.Parallel()

	manifestPath := writeManifest(t, minimalManifest)
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

// TestLoadWalksUpFromWorkDir verifies discovery without an explicit path finds
// the manifest in an ancestor of the work directory.
func TestLoadWalksUpFromWorkDir(t *testing.T) {
	t.Parallel()

	manifestPath := writeManifest(t, minimalManifest)
	nested := filepath.Join(filepath.Dir(manifestPath), "apps", "api")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}

	repo, err := filestore.New(filestore.Params{WorkDir: nested})
	if err != nil {
		t.Fatalf("filestore.New: %v", err)
	}
	ws, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ws.Path != manifestPath {
		t.Errorf("Path = %q, want %q", ws.Path, manifestPath)
	}
}

// TestLoadWalkUpStopsAtGitRoot verifies discovery does not escape the git
// repository containing the work directory.
func TestLoadWalkUpStopsAtGitRoot(t *testing.T) {
	t.Parallel()

	manifestPath := writeManifest(t, minimalManifest)
	repoRoot := filepath.Join(filepath.Dir(manifestPath), "repo")
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o750); err != nil {
		t.Fatal(err)
	}

	repo, err := filestore.New(filestore.Params{WorkDir: repoRoot})
	if err != nil {
		t.Fatalf("filestore.New: %v", err)
	}
	if _, err := repo.Load(); !errors.Is(err, workspace.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}
