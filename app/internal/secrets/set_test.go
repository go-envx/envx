package secrets

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
)

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

// TestSetValidatesBeforeEncryption verifies invalid input and missing identity
// state cannot invoke encryption or create a store file.
func TestSetValidatesBeforeEncryption(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "secrets.yaml")
	cipherDouble := &setTestCipher{}
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
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

// -------------------------------------------------------------------------------------

// TestSetValidatesGeneratedPlaintext verifies a lazy source is invoked after
// public-key lookup and its empty result is rejected before encryption.
func TestSetValidatesGeneratedPlaintext(t *testing.T) {
	t.Parallel()

	storePath := writeStore(t, "public_keys:\n  production: public\n")
	cipherDouble := &setTestCipher{}
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
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

// -------------------------------------------------------------------------------------

// setTestCipher records encryption calls for validation tests.
type setTestCipher struct {
	encryptCalls int
}

// -------------------------------------------------------------------------------------

// Algorithm identifies the algorithm metadata used by the test cipher.
func (setTestCipher) Algorithm() cipher.Algorithm {
	return cipher.Age
}

// -------------------------------------------------------------------------------------

// Keypair returns representative test key material.
func (setTestCipher) Keypair() (cipher.Keypair, error) {
	return cipher.Keypair{PublicKey: "public", PrivateKey: "private"}, nil
}

// -------------------------------------------------------------------------------------

// ValidateKeypair accepts the representative test key material.
func (setTestCipher) ValidateKeypair(string, string) error {
	return nil
}

// -------------------------------------------------------------------------------------

// Encrypt records calls and returns deterministic native ciphertext bytes.
func (c *setTestCipher) Encrypt(string, string) ([]byte, error) {
	c.encryptCalls++
	return []byte("ciphertext"), nil
}

// -------------------------------------------------------------------------------------

// Decrypt returns the deterministic test plaintext.
func (setTestCipher) Decrypt([]byte, string) (string, error) {
	return "plaintext", nil
}
