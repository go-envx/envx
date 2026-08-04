package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------

// writeStore writes body to a secrets.yaml in a fresh temp dir and returns its path.
func writeStore(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// -------------------------------------------------------------------------------------

// TestNewRejectsEmptySecretsPath verifies Manager construction requires a store path.
func TestNewRejectsEmptySecretsPath(t *testing.T) {
	t.Parallel()

	if _, err := New(Params{}); err == nil {
		t.Fatal("New() succeeded without a secrets path")
	}
}

// -------------------------------------------------------------------------------------

// TestNewDefaultsKeysPathAndDependencies verifies the safe local defaults
// selected by New.
func TestNewDefaultsKeysPathAndDependencies(t *testing.T) {
	t.Parallel()

	secretsPath := filepath.Join(t.TempDir(), "secrets.yaml")
	manager, err := New(Params{SecretsPath: secretsPath})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	wantKeysPath := filepath.Join(filepath.Dir(secretsPath), "envx.keys")
	if manager.keysPath != wantKeysPath {
		t.Errorf("keysPath = %q, want %q", manager.keysPath, wantKeysPath)
	}
	if manager.cipher == nil {
		t.Error("cipher is nil")
	}
	if manager.privateKeyResolver == nil {
		t.Error("privateKeyResolver is nil")
	}
	if manager.privateKeyDestination == nil {
		t.Error("privateKeyDestination is nil")
	}
}

// -------------------------------------------------------------------------------------

// TestNewPreservesConfiguredKeysPath verifies an explicit private-key path wins
// over its default.
func TestNewPreservesConfiguredKeysPath(t *testing.T) {
	t.Parallel()

	secretsPath := filepath.Join(t.TempDir(), "secrets.yaml")
	keysPath := filepath.Join(t.TempDir(), "custom.keys")
	manager, err := New(Params{SecretsPath: secretsPath, KeysPath: keysPath})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if manager.keysPath != keysPath {
		t.Errorf("keysPath = %q, want %q", manager.keysPath, keysPath)
	}
}
