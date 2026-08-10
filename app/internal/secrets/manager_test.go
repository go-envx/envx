package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/privatekey"
)

// writeStore writes body to a secrets.yaml in a fresh temp dir and returns its path.
func writeStore(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// newTestCipher creates the default cipher for manager construction tests.
func newTestCipher(t *testing.T) cipher.Cipher {
	t.Helper()
	selected, err := cipher.New(cipher.Params{
		Algorithm: cipher.Age,
		Options:   cipher.AgeOptions{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return selected
}

// newPrivateKeyTestResolver creates a resolver for manager construction tests.
func newPrivateKeyTestResolver() privatekey.Resolver {
	return testResolver{}
}

// newPrivateKeyTestDestination creates a destination for manager construction tests.
func newPrivateKeyTestDestination() privatekey.Destination {
	return testDestination{}
}

// testResolver is a no-op private-key resolver for manager construction tests.
type testResolver struct{}

// Resolve reports that no private key is available.
func (testResolver) Resolve(string) (privatekey.PrivateKey, error) {
	return privatekey.PrivateKey{}, privatekey.ErrNotAvailable
}

// testDestination is a no-op private-key destination for manager construction tests.
type testDestination struct{}

// Write accepts private-key material without storing it.
func (testDestination) Write(string, string) error { return nil }

// TestNewRejectsEmptySecretsPath verifies Manager construction requires a store path.
func TestNewRejectsEmptySecretsPath(t *testing.T) {
	t.Parallel()

	if _, err := New(Params{
		KeysPath:              filepath.Join(t.TempDir(), "envx.keys"),
		Cipher:                newTestCipher(t),
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	}); err == nil {
		t.Fatal("New() succeeded without a secrets path")
	}
}

// TestNewRejectsEmptyKeysPath verifies Manager construction requires a key path.
func TestNewRejectsEmptyKeysPath(t *testing.T) {
	t.Parallel()

	secretsPath := filepath.Join(t.TempDir(), "secrets.yaml")
	if _, err := New(Params{
		SecretsPath:           secretsPath,
		Cipher:                newTestCipher(t),
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	}); err == nil {
		t.Fatal("New() succeeded without a keys path")
	}
}

// TestNewRejectsNilCipher verifies Manager construction requires a cipher.
func TestNewRejectsNilCipher(t *testing.T) {
	t.Parallel()

	params := Params{
		SecretsPath:           filepath.Join(t.TempDir(), "secrets.yaml"),
		KeysPath:              filepath.Join(t.TempDir(), "envx.keys"),
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	}
	if _, err := New(params); err == nil {
		t.Fatal("New() succeeded without a cipher")
	}
}

// TestNewRejectsNilPrivateKeyResolver verifies Manager construction requires
// a resolver.
func TestNewRejectsNilPrivateKeyResolver(t *testing.T) {
	t.Parallel()

	params := Params{
		SecretsPath:           filepath.Join(t.TempDir(), "secrets.yaml"),
		KeysPath:              filepath.Join(t.TempDir(), "envx.keys"),
		Cipher:                newTestCipher(t),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	}
	if _, err := New(params); err == nil || err.Error() != "private-key resolver is nil" {
		t.Fatalf("New() error = %v, want private-key resolver error", err)
	}
}

// TestNewRejectsNilPrivateKeyDestination verifies Manager construction requires
// a destination.
func TestNewRejectsNilPrivateKeyDestination(t *testing.T) {
	t.Parallel()

	params := Params{
		SecretsPath:        filepath.Join(t.TempDir(), "secrets.yaml"),
		KeysPath:           filepath.Join(t.TempDir(), "envx.keys"),
		Cipher:             newTestCipher(t),
		PrivateKeyResolver: newPrivateKeyTestResolver(),
	}
	if _, err := New(params); err == nil ||
		err.Error() != "private-key destination is nil" {
		t.Fatalf("New() error = %v, want private-key destination error", err)
	}
}

// TestNewPreservesConfiguredKeysPath verifies an explicit private-key path is
// retained.
func TestNewPreservesConfiguredKeysPath(t *testing.T) {
	t.Parallel()

	secretsPath := filepath.Join(t.TempDir(), "secrets.yaml")
	keysPath := filepath.Join(t.TempDir(), "custom.keys")
	manager, err := New(Params{
		SecretsPath:           secretsPath,
		KeysPath:              keysPath,
		Cipher:                newTestCipher(t),
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if manager.params.KeysPath != keysPath {
		t.Errorf("keysPath = %q, want %q", manager.params.KeysPath, keysPath)
	}
	if manager.params.Cipher == nil {
		t.Error("cipher is nil")
	}
	if manager.params.PrivateKeyResolver == nil {
		t.Error("privateKeyResolver is nil")
	}
	if manager.params.PrivateKeyDestination == nil {
		t.Error("privateKeyDestination is nil")
	}
}
