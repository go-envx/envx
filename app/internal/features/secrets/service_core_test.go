package secrets

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/features/privatekey"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/value"
)

// fixedPrivateKeyResolver returns one private key for every group.
type fixedPrivateKeyResolver struct {
	// value is the private-key material handed to callers.
	value string
}

// Location returns a test location.
func (r fixedPrivateKeyResolver) Location() string { return "test-keys-location" }

// Resolve returns the fixed private key with a test provenance.
func (r fixedPrivateKeyResolver) Resolve(string) (privatekey.PrivateKey, error) {
	return privatekey.PrivateKey{Value: r.value, Origin: "test"}, nil
}

// Set accepts private-key material without storing it.
func (r fixedPrivateKeyResolver) Set(string, string) error { return nil }

// newGetManager builds a manager whose stored secret decrypts with pair.
func newGetManager(t *testing.T, resolver PrivateKeyService) *Service {
	t.Helper()

	selected := newTestCipher(t)
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	pubKey, privKey := pair.PublicKey, pair.PrivateKey
	if r, ok := resolver.(fixedPrivateKeyResolver); ok && r.value == "" {
		resolver = fixedPrivateKeyResolver{value: privKey}
	}

	storePath := writeStore(t, "public-keys:\n  production: "+pubKey+"\n")
	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, storePath),
		Cipher:            selected,
		PrivateKeyService: resolver,
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
	_, err := manager.SetSecret(
		"production", "database_password", func() (string, error) {
			return plaintext, nil
		},
	)
	if err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	res, err := manager.GetSecret("Production", "database_password")
	if err != nil {
		t.Fatalf("GetSecret(): %v", err)
	}
	if res.Value != plaintext {
		t.Errorf("GetSecret() = %q, want %q", res.Value, plaintext)
	}
}

// TestGetMissingSecretFails verifies a dangling reference is an error.
func TestGetMissingSecretFails(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, fixedPrivateKeyResolver{})

	if _, err := manager.GetSecret("production", "missing"); err == nil {
		t.Fatal("GetSecret() succeeded for a missing secret")
	}
}

// TestGetUnavailablePrivateKeyFails verifies Get fails when no private key is
// available, since it promises plaintext.
func TestGetUnavailablePrivateKeyFails(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	_, err := manager.SetSecret(
		"production", "database_password", func() (string, error) {
			return "database-password", nil
		},
	)
	if err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}
	if _, err := manager.GetSecret("production", "database_password"); err == nil {
		t.Fatal("GetSecret() succeeded without an available private key")
	}
}

// TestGetRejectsInvalidInput verifies Get validates its group and key.
func TestGetRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, fixedPrivateKeyResolver{})

	if _, err := manager.GetSecret("", "key"); err == nil {
		t.Error("GetSecret() accepted an empty group")
	}
	if _, err := manager.GetSecret("production", ""); err == nil {
		t.Error("GetSecret() accepted an empty key")
	}
}

// TestHasReportsPresence verifies Has detects stored entries case-insensitively
// by group without loading a private key.
func TestHasReportsPresence(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	_, err := manager.SetSecret(
		"production", "database_password", func() (string, error) {
			return "database-password", nil
		},
	)
	if err != nil {
		t.Fatalf("SetSecret(): %v", err)
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
	pubKey, privKey := pair.PublicKey, pair.PrivateKey
	storePath := writeStore(t, "public-keys:\n  production: "+pubKey+"\n")
	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, storePath),
		Cipher:            selected,
		PrivateKeyService: newPrivateKeyTestService(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	const plaintext = "database-password"
	_, err = manager.SetSecret(
		"Production", "database_password", func() (string, error) {
			return plaintext, nil
		},
	)
	if err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	secret, exists, err := manager.params.Repository.GetSecret(
		"production", "database_password",
	)
	if err != nil {
		t.Fatalf("GetSecret() after Set: %v", err)
	}
	if !exists {
		t.Fatal("SetSecret() did not store the secret")
	}
	if strings.Contains(secret.Ciphertext, plaintext) {
		t.Fatalf("stored value contains plaintext: %q", secret.Ciphertext)
	}
	codec := newEnvelopeCodec(envelopeParams{})
	algorithm, nativeCiphertext, err := codec.decode(secret.Ciphertext)
	if err != nil {
		t.Fatalf("Decode(): %v", err)
	}
	if algorithm != "age" {
		t.Fatalf("envelope algorithm = %q, want age", algorithm)
	}
	got, err := selected.Decrypt(nativeCiphertext, privKey)
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
	pubKey, privKey := pair.PublicKey, pair.PrivateKey
	storePath := writeStore(t, "public-keys:\n  shared: "+pubKey+"\n")
	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, storePath),
		Cipher:            selected,
		PrivateKeyService: newPrivateKeyTestService(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	const plaintext = "shared-token"
	if _, err := manager.SetSecret("SHARED", "service_token", func() (string, error) {
		return plaintext, nil
	}); err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}
	secret, exists, err := manager.params.Repository.GetSecret("shared", "service_token")
	if err != nil {
		t.Fatalf("GetSecret(): %v", err)
	}
	if !exists {
		t.Fatal("Set() did not store the secret")
	}
	codec := newEnvelopeCodec(envelopeParams{})
	algorithm, nativeCiphertext, err := codec.decode(secret.Ciphertext)
	if err != nil {
		t.Fatalf("Decode(): %v", err)
	}
	if algorithm != "nacl-box" {
		t.Fatalf("envelope algorithm = %q, want nacl-box", algorithm)
	}
	got, err := selected.Decrypt(nativeCiphertext, privKey)
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
	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, storePath),
		Cipher:            cipherDouble,
		PrivateKeyService: newPrivateKeyTestService(),
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
			_, err := manager.SetSecret(test.group, test.key, func() (string, error) {
				sourceCalls++
				return test.plaintext, nil
			})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("SetSecret() error = %v, want %q", err, test.wantErr)
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

	storePath := writeStore(t, "public-keys:\n  production: public\n")
	cipherDouble := &setTestCipher{}
	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, storePath),
		Cipher:            cipherDouble,
		PrivateKeyService: newPrivateKeyTestService(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	sourceCalls := 0
	_, err = manager.SetSecret("production", "key", func() (string, error) {
		sourceCalls++
		return "", nil
	})
	if err == nil || !strings.Contains(err.Error(), "secret plaintext is empty") {
		t.Fatalf("SetSecret() error = %v, want empty plaintext error", err)
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
func (setTestCipher) Algorithm() string {
	return "age"
}

// Keypair returns representative test key material.
func (setTestCipher) Keypair() (value.Keypair, error) {
	return value.Keypair{PublicKey: "public", PrivateKey: "private"}, nil
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
		if _, err := manager.SetSecret("production", key, func() (string, error) {
			return "value", nil
		}); err != nil {
			t.Fatalf("SetSecret(%q): %v", key, err)
		}
	}
	set("database_password")
	set("service_token")

	result, err := manager.DeleteSecret("Production", "database_password")
	if err != nil {
		t.Fatalf("DeleteSecret(): %v", err)
	}
	if result.Secret.Group != "production" || result.Secret.Key != "database_password" {
		t.Errorf("result = %+v", result)
	}

	exists, err := manager.Has("production", "database_password")
	if err != nil {
		t.Fatalf("Has() deleted: %v", err)
	}
	if exists {
		t.Error("DeleteSecret() left the removed secret in the store")
	}
	exists, err = manager.Has("production", "service_token")
	if err != nil {
		t.Fatalf("Has() sibling: %v", err)
	}
	if !exists {
		t.Error("DeleteSecret() removed an unrelated secret")
	}
}

// TestDeletePreservesGroupIdentity verifies removing a group's last value keeps
// its public key so the identity is not torn down implicitly.
func TestDeletePreservesGroupIdentity(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	if _, err := manager.SetSecret("production", "only", func() (string, error) {
		return "value", nil
	}); err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}
	if _, err := manager.DeleteSecret("production", "only"); err != nil {
		t.Fatalf("DeleteSecret(): %v", err)
	}

	_, exists, err := manager.params.Repository.GetPublicKey("production")
	if err != nil || !exists {
		t.Error("DeleteSecret() removed the group's public key")
	}
}

// TestDeleteMissingSecretFails verifies deleting an absent entry is an error.
func TestDeleteMissingSecretFails(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	if _, err := manager.DeleteSecret("production", "missing"); err == nil {
		t.Fatal("DeleteSecret() succeeded for a missing secret")
	}
}

// TestDeleteRejectsInvalidInput verifies Delete validates its group and key.
func TestDeleteRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())

	if _, err := manager.DeleteSecret("", "key"); err == nil {
		t.Error("DeleteSecret() accepted an empty group")
	}
	if _, err := manager.DeleteSecret("production", ""); err == nil {
		t.Error("DeleteSecret() accepted an empty key")
	}
}

// TestValidateSecretKey verifies secret key identifier syntax.
func TestValidateSecretKey(t *testing.T) {
	t.Parallel()

	for _, key := range []string{
		"valid-key",
		"API_KEY",
		"secret.value",
	} {
		if err := ValidateSecretKey(key); err != nil {
			t.Errorf("ValidateSecretKey(%q) error = %v, want nil", key, err)
		}
	}

	for _, invalid := range []string{
		"",
		"   ",
		"key/with/slash",
		"key\nwith\nnewline",
		"key\rwith\rcarriage",
	} {
		if err := ValidateSecretKey(invalid); err == nil {
			t.Errorf("ValidateSecretKey(%q) = nil, want error", invalid)
		}
	}
}

// TestNormalizeGroupName verifies secret group normalization and syntax.
func TestNormalizeGroupName(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input string
		want  string
	}{
		{"production", "production"},
		{"Production", "production"},
		{"SHARED", "shared"},
		{"group-1", "group-1"},
	} {
		got, err := NormalizeGroupName(tc.input)
		if err != nil {
			t.Errorf("normalizeGroupName(%q) error = %v, want nil", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("normalizeGroupName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}

	for _, invalid := range []string{
		"",
		"   ",
		"group with space",
		"group/slash",
		"group=equals",
		"group\nnewline",
	} {
		if _, err := NormalizeGroupName(invalid); err == nil {
			t.Errorf("normalizeGroupName(%q) = nil, want error", invalid)
		}
	}
}
