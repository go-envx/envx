package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/features/privatekey"
	"github.com/go-envx/envx/app/internal/resources/cipher"
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

// newPrivateKeyTestService creates a service double for manager construction tests.
func newPrivateKeyTestService() PrivateKeyService {
	return testPrivateKeyService{}
}

// newPrivateKeyTestResolver creates a service double for test helpers.
func newPrivateKeyTestResolver() PrivateKeyService {
	return testPrivateKeyService{}
}

// testPrivateKeyService is a no-op private-key service for manager construction tests.
type testPrivateKeyService struct{}

// Resolve reports that no private key is available.
func (testPrivateKeyService) Resolve(string) (privatekey.PrivateKey, error) {
	return privatekey.PrivateKey{}, privatekey.ErrNotAvailable
}

// Set accepts private-key material without storing it.
func (testPrivateKeyService) Set(string, string) error { return nil }

// TestNewRejectsEmptySecretsPath verifies Manager construction requires a store path.
func TestNewRejectsEmptySecretsPath(t *testing.T) {
	t.Parallel()

	if _, err := New(Params{
		KeysPath:          filepath.Join(t.TempDir(), "envx.keys"),
		Cipher:            newTestCipher(t),
		PrivateKeyService: newPrivateKeyTestService(),
	}); err == nil {
		t.Fatal("New() succeeded without a secrets path")
	}
}

// TestNewRejectsEmptyKeysPath verifies Manager construction requires a key path.
func TestNewRejectsEmptyKeysPath(t *testing.T) {
	t.Parallel()

	secretsPath := filepath.Join(t.TempDir(), "secrets.yaml")
	if _, err := New(Params{
		SecretsPath:       secretsPath,
		Cipher:            newTestCipher(t),
		PrivateKeyService: newPrivateKeyTestService(),
	}); err == nil {
		t.Fatal("New() succeeded without a keys path")
	}
}

// TestNewRejectsNilCipher verifies Manager construction requires a cipher.
func TestNewRejectsNilCipher(t *testing.T) {
	t.Parallel()

	params := Params{
		SecretsPath:       filepath.Join(t.TempDir(), "secrets.yaml"),
		KeysPath:          filepath.Join(t.TempDir(), "envx.keys"),
		PrivateKeyService: newPrivateKeyTestService(),
	}
	if _, err := New(params); err == nil {
		t.Fatal("New() succeeded without a cipher")
	}
}

// TestNewRejectsNilPrivateKeyService verifies Manager construction requires
// a private-key service.
func TestNewRejectsNilPrivateKeyService(t *testing.T) {
	t.Parallel()

	params := Params{
		SecretsPath: filepath.Join(t.TempDir(), "secrets.yaml"),
		KeysPath:    filepath.Join(t.TempDir(), "envx.keys"),
		Cipher:      newTestCipher(t),
	}
	if _, err := New(params); err == nil || err.Error() != "private-key service is nil" {
		t.Fatalf("New() error = %v, want private-key service error", err)
	}
}

// TestNewRejectsInvalidDefaultIndent verifies Manager construction requires a
// usable default indentation from the configuration layer.
func TestNewRejectsInvalidDefaultIndent(t *testing.T) {
	t.Parallel()

	params := Params{
		SecretsPath:       filepath.Join(t.TempDir(), "secrets.yaml"),
		KeysPath:          filepath.Join(t.TempDir(), "envx.keys"),
		Cipher:            newTestCipher(t),
		PrivateKeyService: newPrivateKeyTestService(),
	}
	if _, err := New(params); err == nil ||
		err.Error() != "default indent must be at least 2 spaces" {
		t.Fatalf("New() error = %v, want default indent error", err)
	}
}

// TestNewPreservesConfiguredKeysPath verifies an explicit private-key path is
// retained.
func TestNewPreservesConfiguredKeysPath(t *testing.T) {
	t.Parallel()

	secretsPath := filepath.Join(t.TempDir(), "secrets.yaml")
	keysPath := filepath.Join(t.TempDir(), "custom.keys")
	manager, err := New(Params{
		SecretsPath:       secretsPath,
		KeysPath:          keysPath,
		DefaultIndent:     2,
		Cipher:            newTestCipher(t),
		PrivateKeyService: newPrivateKeyTestService(),
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
	if manager.params.PrivateKeyService == nil {
		t.Error("privateKeyService is nil")
	}
}
