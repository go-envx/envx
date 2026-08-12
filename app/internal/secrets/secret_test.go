package secrets

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/privatekey"
	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
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

// TestSetEncryptsAndStoresSecret verifies Set writes an envelope that decrypts
// back to the supplied plaintext without storing that plaintext.
func TestSetEncryptsAndStoresSecret(t *testing.T) {
	t.Parallel()

	selected := newTestCipher(t)
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	storePath := writeStore(t, "public_keys:\n  production: "+pair.PublicKey+"\n")
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
		DefaultIndent:         2,
		Cipher:                selected,
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	const plaintext = "database-password"
	if err := manager.Set("Production", "database_password", func() (string, error) {
		return plaintext, nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}

	document, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("Open() after Set: %v", err)
	}
	secret, exists := document.Secret("production", "database_password")
	if !exists {
		t.Fatal("Set() did not store the secret")
	}
	if strings.Contains(secret.Value, plaintext) {
		t.Fatalf("stored value contains plaintext: %q", secret.Value)
	}
	algorithm, nativeCiphertext, err := envelope.Decode(secret.Value)
	if err != nil {
		t.Fatalf("Decode(): %v", err)
	}
	if algorithm != cipher.Age {
		t.Fatalf("envelope algorithm = %q, want %q", algorithm, cipher.Age)
	}
	got, err := selected.Decrypt(nativeCiphertext, pair.PrivateKey)
	if err != nil {
		t.Fatalf("Decrypt(): %v", err)
	}
	if got != plaintext {
		t.Errorf("round trip = %q, want %q", got, plaintext)
	}
}

// TestSetUsesCipherAlgorithm verifies a configured NaCl Box cipher receives its
// own envelope tag and can decrypt the stored value.
func TestSetUsesCipherAlgorithm(t *testing.T) {
	t.Parallel()

	selected, err := cipher.New(cipher.Params{Algorithm: cipher.NaClBox})
	if err != nil {
		t.Fatalf("cipher.New(): %v", err)
	}
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	storePath := writeStore(t, "public_keys:\n  shared: "+pair.PublicKey+"\n")
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
		DefaultIndent:         2,
		Cipher:                selected,
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	const plaintext = "shared-token"
	if err := manager.Set("SHARED", "service_token", func() (string, error) {
		return plaintext, nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}
	document, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	secret, exists := document.Secret("shared", "service_token")
	if !exists {
		t.Fatal("Set() did not store the secret")
	}
	algorithm, nativeCiphertext, err := envelope.Decode(secret.Value)
	if err != nil {
		t.Fatalf("Decode(): %v", err)
	}
	if algorithm != cipher.NaClBox {
		t.Fatalf("envelope algorithm = %q, want %q", algorithm, cipher.NaClBox)
	}
	got, err := selected.Decrypt(nativeCiphertext, pair.PrivateKey)
	if err != nil {
		t.Fatalf("Decrypt(): %v", err)
	}
	if got != plaintext {
		t.Errorf("round trip = %q, want %q", got, plaintext)
	}
}

// TestSetValidatesBeforeEncryption verifies invalid input and missing identity
// state cannot invoke encryption or create a store file.
func TestSetValidatesBeforeEncryption(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "secrets.yaml")
	cipherDouble := &setTestCipher{}
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
		DefaultIndent:         2,
		Cipher:                cipherDouble,
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	for _, test := range []struct {
		name       string
		group      string
		key        string
		plaintext  string
		wantErr    string
		wantSource int
		wantCalled int
	}{
		{
			name:      "empty group",
			key:       "key",
			plaintext: "value",
			wantErr:   "secret group is empty",
		},
		{
			name:      "empty key",
			group:     "production",
			plaintext: "value",
			wantErr:   "secret key is empty",
		},
		{
			name:       "empty plaintext without identity",
			group:      "production",
			key:        "key",
			wantErr:    "has no public key",
			wantSource: 0,
		},
		{
			name:       "missing public key",
			group:      "production",
			key:        "key",
			plaintext:  "value",
			wantErr:    "has no public key",
			wantSource: 0,
			wantCalled: 0,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			sourceCalls := 0
			before := cipherDouble.encryptCalls
			err := manager.Set(test.group, test.key, func() (string, error) {
				sourceCalls++
				return test.plaintext, nil
			})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Set() error = %v, want %q", err, test.wantErr)
			}
			if sourceCalls != test.wantSource {
				t.Errorf("plaintext source calls = %d, want %d", sourceCalls, test.wantSource)
			}
			if got := cipherDouble.encryptCalls - before; got != test.wantCalled {
				t.Errorf("Encrypt() calls = %d, want %d", got, test.wantCalled)
			}
		})
	}
}

// TestSetValidatesGeneratedPlaintext verifies a lazy source is invoked after
// public-key lookup and its empty result is rejected before encryption.
func TestSetValidatesGeneratedPlaintext(t *testing.T) {
	t.Parallel()

	storePath := writeStore(t, "public_keys:\n  production: public\n")
	cipherDouble := &setTestCipher{}
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
		DefaultIndent:         2,
		Cipher:                cipherDouble,
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	sourceCalls := 0
	err = manager.Set("production", "key", func() (string, error) {
		sourceCalls++
		return "", nil
	})
	if err == nil || !strings.Contains(err.Error(), "secret plaintext is empty") {
		t.Fatalf("Set() error = %v, want empty plaintext error", err)
	}
	if sourceCalls != 1 {
		t.Errorf("plaintext source calls = %d, want 1", sourceCalls)
	}
	if cipherDouble.encryptCalls != 0 {
		t.Errorf("Encrypt() calls = %d, want 0", cipherDouble.encryptCalls)
	}
}

// setTestCipher records encryption calls for validation tests.
type setTestCipher struct {
	encryptCalls int
}

// Algorithm identifies the algorithm metadata used by the test cipher.
func (setTestCipher) Algorithm() cipher.Algorithm {
	return cipher.Age
}

// Keypair returns representative test key material.
func (setTestCipher) Keypair() (cipher.Keypair, error) {
	return cipher.Keypair{PublicKey: "public", PrivateKey: "private"}, nil
}

// ValidateKeypair accepts the representative test key material.
func (setTestCipher) ValidateKeypair(string, string) error {
	return nil
}

// Encrypt records calls and returns deterministic native ciphertext bytes.
func (c *setTestCipher) Encrypt(string, string) ([]byte, error) {
	c.encryptCalls++
	return []byte("ciphertext"), nil
}

// Decrypt returns the deterministic test plaintext.
func (setTestCipher) Decrypt([]byte, string) (string, error) {
	return "plaintext", nil
}

// TestDeleteRemovesStoredSecret verifies Delete removes one value matched by a
// case-insensitive group and leaves other values in place.
func TestDeleteRemovesStoredSecret(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	set := func(key string) {
		if err := manager.Set("production", key, func() (string, error) {
			return "value", nil
		}); err != nil {
			t.Fatalf("Set(%q): %v", key, err)
		}
	}
	set("database_password")
	set("service_token")

	if err := manager.Delete("Production", "database_password"); err != nil {
		t.Fatalf("Delete(): %v", err)
	}

	exists, err := manager.Has("production", "database_password")
	if err != nil {
		t.Fatalf("Has() deleted: %v", err)
	}
	if exists {
		t.Error("Delete() left the removed secret in the store")
	}
	exists, err = manager.Has("production", "service_token")
	if err != nil {
		t.Fatalf("Has() sibling: %v", err)
	}
	if !exists {
		t.Error("Delete() removed an unrelated secret")
	}
}

// TestDeletePreservesGroupIdentity verifies removing a group's last value keeps
// its public key so the identity is not torn down implicitly.
func TestDeletePreservesGroupIdentity(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	if err := manager.Set("production", "only", func() (string, error) {
		return "value", nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}
	if err := manager.Delete("production", "only"); err != nil {
		t.Fatalf("Delete(): %v", err)
	}

	document, err := store.Open(manager.params.SecretsPath)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	if _, exists := document.PublicKey("production"); !exists {
		t.Error("Delete() removed the group's public key")
	}
}

// TestDeleteMissingSecretFails verifies deleting an absent entry is an error.
func TestDeleteMissingSecretFails(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	if err := manager.Delete("production", "missing"); err == nil {
		t.Fatal("Delete() succeeded for a missing secret")
	}
}

// TestDeleteRejectsInvalidInput verifies Delete validates its group and key.
func TestDeleteRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	if err := manager.Delete("", "key"); err == nil {
		t.Error("Delete() accepted an empty group")
	}
	if err := manager.Delete("production", ""); err == nil {
		t.Error("Delete() accepted an empty key")
	}
}
