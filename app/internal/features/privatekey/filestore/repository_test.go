package filestore

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/features/privatekey"
)

func TestNewDefaultsEmptyPath(t *testing.T) {
	t.Parallel()

	store, err := New(Params{Path: ""})
	if err != nil {
		t.Fatalf("New() with empty path failed: %v", err)
	}
	if store.path != defaultFilename {
		t.Errorf("store.path = %q, want %q", store.path, defaultFilename)
	}

	store, err = New(Params{Path: "   "})
	if err != nil {
		t.Fatalf("New() with whitespace path failed: %v", err)
	}
	if store.path != defaultFilename {
		t.Errorf("store.path = %q, want %q", store.path, defaultFilename)
	}
}

func TestRepositoryGetPrivateKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "envx.keys")
	content := "# comment\nPRODUCTION=prod-secret\nSTAGING=staging-secret\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	repo, err := New(Params{Path: path})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	key, found, err := repo.GetPrivateKey("production")
	if err != nil {
		t.Fatalf("GetPrivateKey(production): %v", err)
	}
	if !found || key != "prod-secret" {
		t.Errorf("GetPrivateKey(production) = (%q, %v), want (prod-secret, true)", key, found)
	}

	// Case insensitive
	key, found, err = repo.GetPrivateKey("STAGING")
	if err != nil {
		t.Fatalf("GetPrivateKey(STAGING): %v", err)
	}
	if !found || key != "staging-secret" {
		t.Errorf("GetPrivateKey(STAGING) = (%q, %v), want (staging-secret, true)", key, found)
	}

	// Missing group
	key, found, err = repo.GetPrivateKey("missing")
	if err != nil {
		t.Fatalf("GetPrivateKey(missing): %v", err)
	}
	if found || key != "" {
		t.Errorf("GetPrivateKey(missing) = (%q, %v), want ('', false)", key, found)
	}

	// Invalid group
	_, _, err = repo.GetPrivateKey("invalid group")
	if err == nil {
		t.Fatal("GetPrivateKey(invalid group) accepted whitespace in group name")
	}
}

func TestRepositoryGetPrivateKeyMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo, err := New(Params{Path: filepath.Join(dir, "nonexistent.keys")})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	key, found, err := repo.GetPrivateKey("production")
	if err != nil {
		t.Fatalf("GetPrivateKey(): unexpected error %v", err)
	}
	if found || key != "" {
		t.Errorf("GetPrivateKey() = (%q, %v), want ('', false)", key, found)
	}
}

func TestRepositoryGetPrivateKeyMalformedFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "envx.keys")
	if err := os.WriteFile(path, []byte("not-an-entry\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	repo, err := New(Params{Path: path})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	_, _, err = repo.GetPrivateKey("production")
	if !errors.Is(err, privatekey.ErrInvalidKey) {
		t.Fatalf("GetPrivateKey() error = %v, want ErrInvalidKey", err)
	}
}

func TestRepositorySetPrivateKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "envx.keys")

	repo, err := New(Params{Path: path})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	// Write new key into nonexistent file in nested directory
	if err := repo.SetPrivateKey("production", "prod-val-1"); err != nil {
		t.Fatalf("SetPrivateKey(production): %v", err)
	}

	// Verify file mode is 0600
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, want 0600", info.Mode().Perm())
	}

	// Update existing key
	if err := repo.SetPrivateKey("production", "prod-val-2"); err != nil {
		t.Fatalf("SetPrivateKey(production) update: %v", err)
	}

	// Add second group
	if err := repo.SetPrivateKey("staging", "staging-val"); err != nil {
		t.Fatalf("SetPrivateKey(staging): %v", err)
	}

	k1, found, err := repo.GetPrivateKey("production")
	if err != nil || !found || k1 != "prod-val-2" {
		t.Errorf("GetPrivateKey(production) = (%q, %v), want (prod-val-2, true)", k1, found)
	}
	k2, found, err := repo.GetPrivateKey("staging")
	if err != nil || !found || k2 != "staging-val" {
		t.Errorf("GetPrivateKey(staging) = (%q, %v), want (staging-val, true)", k2, found)
	}
}

func TestRepositorySetPrivateKeyRejectsInvalidEntry(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo, err := New(Params{Path: filepath.Join(dir, "envx.keys")})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	tests := []struct {
		name       string
		group      string
		privateKey string
	}{
		{name: "empty group", group: "", privateKey: "key"},
		{name: "whitespace group", group: "  ", privateKey: "key"},
		{name: "group with equals", group: "A=B", privateKey: "key"},
		{name: "empty private key", group: "production", privateKey: ""},
		{name: "newline in private key", group: "production", privateKey: "a\nb"},
		{name: "carriage return in private key", group: "production", privateKey: "a\rb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := repo.SetPrivateKey(tt.group, tt.privateKey); err == nil {
				t.Fatalf("SetPrivateKey(%q, %q) accepted invalid input", tt.group, tt.privateKey)
			}
		})
	}
}

func TestRepositorySetPrivateKeyRejectsMalformedExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "envx.keys")
	content := []byte("not-an-entry\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	repo, err := New(Params{Path: path})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	if err := repo.SetPrivateKey("production", "private-value"); err == nil {
		t.Fatal("SetPrivateKey() accepted malformed existing content")
	} else if !errors.Is(err, privatekey.ErrInvalidKey) {
		t.Errorf("SetPrivateKey() error = %v, want ErrInvalidKey", err)
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is test-local.
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("file contents = %q, want unchanged content %q", data, content)
	}
}
