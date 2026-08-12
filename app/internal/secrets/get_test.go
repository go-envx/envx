package secrets

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/privatekey"
)

// fixedPrivateKeyResolver returns one private key for every group.
type fixedPrivateKeyResolver struct {
	// value is the private-key material handed to callers.
	value string
}

// Resolve returns the fixed private key with a test provenance.
func (r fixedPrivateKeyResolver) Resolve(string) (privatekey.PrivateKey, error) {
	return privatekey.PrivateKey{Value: r.value, Origin: "test"}, nil
}

// newGetManager builds a manager whose stored secret decrypts with pair.
func newGetManager(t *testing.T, resolver privatekey.Resolver) *Manager {
	t.Helper()

	selected := newTestCipher(t)
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	if r, ok := resolver.(fixedPrivateKeyResolver); ok && r.value == "" {
		resolver = fixedPrivateKeyResolver{value: pair.PrivateKey}
	}

	storePath := writeStore(t, "public_keys:\n  production: "+pair.PublicKey+"\n")
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
		DefaultIndent:         2,
		Cipher:                selected,
		PrivateKeyResolver:    resolver,
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return manager
}

// TestGetDecryptsStoredSecret verifies Get returns the plaintext of a value
// stored under a case-insensitive group.
func TestGetDecryptsStoredSecret(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, fixedPrivateKeyResolver{})

	const plaintext = "database-password"
	if err := manager.Set("production", "database_password", func() (string, error) {
		return plaintext, nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}

	got, err := manager.Get("Production", "database_password")
	if err != nil {
		t.Fatalf("Get(): %v", err)
	}
	if got != plaintext {
		t.Errorf("Get() = %q, want %q", got, plaintext)
	}
}

// TestGetMissingSecretFails verifies a dangling reference is an error.
func TestGetMissingSecretFails(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, fixedPrivateKeyResolver{})

	if _, err := manager.Get("production", "missing"); err == nil {
		t.Fatal("Get() succeeded for a missing secret")
	}
}

// TestGetUnavailablePrivateKeyFails verifies Get fails when no private key is
// available, since it promises plaintext.
func TestGetUnavailablePrivateKeyFails(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	if err := manager.Set("production", "database_password", func() (string, error) {
		return "database-password", nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}
	if _, err := manager.Get("production", "database_password"); err == nil {
		t.Fatal("Get() succeeded without an available private key")
	}
}

// TestGetRejectsInvalidInput verifies Get validates its group and key.
func TestGetRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, fixedPrivateKeyResolver{})

	if _, err := manager.Get("", "key"); err == nil {
		t.Error("Get() accepted an empty group")
	}
	if _, err := manager.Get("production", ""); err == nil {
		t.Error("Get() accepted an empty key")
	}
}

// TestHasReportsPresence verifies Has detects stored entries case-insensitively
// by group without loading a private key.
func TestHasReportsPresence(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	if err := manager.Set("production", "database_password", func() (string, error) {
		return "database-password", nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}

	exists, err := manager.Has("Production", "database_password")
	if err != nil {
		t.Fatalf("Has(): %v", err)
	}
	if !exists {
		t.Error("Has() = false for a stored secret")
	}

	exists, err = manager.Has("production", "missing")
	if err != nil {
		t.Fatalf("Has() for missing key: %v", err)
	}
	if exists {
		t.Error("Has() = true for a missing secret")
	}
}

// TestHasRejectsInvalidInput verifies Has validates its group and key.
func TestHasRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	if _, err := manager.Has("", "key"); err == nil {
		t.Error("Has() accepted an empty group")
	}
	if _, err := manager.Has("production", ""); err == nil {
		t.Error("Has() accepted an empty key")
	}
}
