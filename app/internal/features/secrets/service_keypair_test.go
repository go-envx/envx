package secrets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/features/privatekey"
	pkfilestore "github.com/go-envx/envx/app/internal/features/privatekey/filestore"
	"github.com/go-envx/envx/app/internal/resources/cipher"
)

// keypairTestCipher is a deterministic cipher double for keypair workflow tests.
type keypairTestCipher struct {
	// pair is returned by Keypair.
	pair cipher.Keypair
	// validPrivate is the only private key ValidateKeypair accepts.
	validPrivate string
}

// Algorithm identifies the algorithm metadata used by the test cipher.
func (keypairTestCipher) Algorithm() string {
	return "age"
}

// Keypair returns the deterministic test keypair.
func (c keypairTestCipher) Keypair() (publicKey, privateKey string, err error) {
	return c.pair.PublicKey, c.pair.PrivateKey, nil
}

// ValidateKeypair validates the deterministic test keypair.
func (c keypairTestCipher) ValidateKeypair(publicKey, privateKey string) error {
	if publicKey != c.pair.PublicKey || privateKey != c.validPrivate {
		return errors.New("test keypair mismatch")
	}
	return nil
}

// Encrypt is unused by keypair workflow tests.
func (keypairTestCipher) Encrypt(string, string) ([]byte, error) {
	return nil, errors.New("test cipher encryption is unused")
}

// Decrypt is unused by keypair workflow tests.
func (keypairTestCipher) Decrypt([]byte, string) (string, error) {
	return "", errors.New("test cipher decryption is unused")
}

// keypairTestService provides a test double implementing PrivateKeyService.
type keypairTestService struct {
	key      privatekey.PrivateKey
	err      error
	write    func(group, privateKey string) error
	location string
}

func (s keypairTestService) Location() string {
	if s.location != "" {
		return s.location
	}
	return "test-keys-location"
}

func (s keypairTestService) Resolve(string) (privatekey.PrivateKey, error) {
	return s.key, s.err
}

func (s keypairTestService) Set(group, privateKey string) error {
	if s.write != nil {
		return s.write(group, privateKey)
	}
	return nil
}

// TestGenerateKeypairCommitsPublicStateAfterPrivateHandoff verifies the safe
// write order and that the result does not carry private material.
func TestGenerateKeypairCommitsPublicStateAfterPrivateHandoff(t *testing.T) {
	t.Parallel()

	const privateValue = "private-test-value"
	cipherDouble := keypairTestCipher{
		pair: cipher.Keypair{
			PublicKey:  "public-test-value",
			PrivateKey: privateValue,
		},
		validPrivate: privateValue,
	}
	repo := newFakeRepository()
	service := keypairTestService{
		write: func(group, privateKey string) error {
			if group != "production" || privateKey != privateValue {
				t.Fatalf("destination received (%q, %q)", group, privateKey)
			}
			if _, exists, _ := repo.GetPublicKey(group); exists {
				return errors.New("public key was committed before private handoff")
			}
			return nil
		},
	}

	manager, err := NewService(ServiceParams{
		Repository:        repo,
		Cipher:            cipherDouble,
		PrivateKeyService: service,
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	result, err := manager.GenerateKeypair("production")
	if err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if result.Keypair.PrivateKeyStatus != PrivateKeyValid {
		t.Errorf(
			"PrivateKeyStatus = %q, want %q",
			result.Keypair.PrivateKeyStatus,
			PrivateKeyValid,
		)
	}
	if strings.Contains(fmt.Sprintf("%+v", result.Keypair), privateValue) {
		t.Fatal("keypair metadata contains private material")
	}

	got, exists, err := repo.GetPublicKey("PRODUCTION")
	if err != nil {
		t.Fatalf("GetPublicKey() after generation: %v", err)
	}
	if !exists || got != "public-test-value" {
		t.Errorf("PublicKey() = (%q, %v)", got, exists)
	}
}

// TestGenerateKeypairRefusesExistingIdentity verifies generation never replaces
// a group's existing public identity.
func TestGenerateKeypairRefusesExistingIdentity(t *testing.T) {
	t.Parallel()

	storePath := writeStore(t, "public-keys:\n  production: existing-public\n")
	called := false
	manager, err := NewService(ServiceParams{
		Repository: newTestStore(t, storePath),
		Cipher: keypairTestCipher{
			pair:         cipher.Keypair{PublicKey: "new-public", PrivateKey: "new-private"},
			validPrivate: "new-private",
		},
		PrivateKeyService: keypairTestService{
			write: func(string, string) error {
				called = true
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if _, err := manager.GenerateKeypair("production"); err == nil {
		t.Fatal("GenerateKeypair() succeeded for an existing identity")
	}
	if called {
		t.Fatal("destination was called for an existing identity")
	}
}

// TestInspectKeypairStatuses verifies unavailable, valid, and invalid private
// key states without returning private material.
func TestInspectKeypairStatuses(t *testing.T) {
	t.Parallel()

	storePath := writeStore(t, "public-keys:\n  production: public-test-value\n")
	cipherDouble := keypairTestCipher{
		pair: cipher.Keypair{
			PublicKey:  "public-test-value",
			PrivateKey: "private-test-value",
		},
		validPrivate: "private-test-value",
	}
	tests := []struct {
		name    string
		service PrivateKeyService
		want    PrivateKeyStatus
	}{
		{
			name: "unavailable",
			service: keypairTestService{
				err: fmt.Errorf("%w for group production", privatekey.ErrNotAvailable),
			},
			want: PrivateKeyNotAvailable,
		},
		{
			name: "valid",
			service: keypairTestService{
				key: privatekey.PrivateKey{Value: "private-test-value", Origin: "test"},
			},
			want: PrivateKeyValid,
		},
		{
			name: "invalid",
			service: keypairTestService{
				key: privatekey.PrivateKey{Value: "wrong-private-value", Origin: "test"},
			},
			want: PrivateKeyInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			manager, err := NewService(ServiceParams{
				Repository:        newTestStore(t, storePath),
				Cipher:            cipherDouble,
				PrivateKeyService: tt.service,
			})
			if err != nil {
				t.Fatalf("New(): %v", err)
			}
			metadata, err := manager.InspectKeypair("production")
			if err != nil {
				t.Fatalf("InspectKeypair(): %v", err)
			}
			if metadata.PrivateKeyStatus != tt.want {
				t.Errorf("PrivateKeyStatus = %q, want %q", metadata.PrivateKeyStatus, tt.want)
			}
			if strings.Contains(fmt.Sprintf("%+v", metadata), "private-test-value") ||
				strings.Contains(fmt.Sprintf("%+v", metadata), "wrong-private-value") {
				t.Fatal("keypair metadata contains private material")
			}
		})
	}
}

// TestGenerateDefaultKeypairRoundTrip verifies the default age cipher, local
// key-file destination, private permissions, and local Git ignore protection.
func TestGenerateDefaultKeypairRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "secrets.yaml")
	keysPath := filepath.Join(dir, "envx.keys")
	pkStore, err := pkfilestore.New(pkfilestore.Params{Path: keysPath})
	if err != nil {
		t.Fatalf("pkfilestore.New: %v", err)
	}
	pkService, err := privatekey.NewService(privatekey.ServiceParams{
		Repository: pkStore,
		LookupEnv:  func(string) (string, bool) { return "", false },
	})
	if err != nil {
		t.Fatalf("privatekey.NewService: %v", err)
	}
	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, storePath),
		Cipher:            newTestCipher(t),
		PrivateKeyService: pkService,
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	result, err := manager.GenerateKeypair("production")
	if err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if result.Keypair.PrivateKeyStatus != PrivateKeyValid {
		t.Errorf(
			"PrivateKeyStatus = %q, want %q",
			result.Keypair.PrivateKeyStatus,
			PrivateKeyValid,
		)
	}

	info, err := os.Stat(keysPath)
	if err != nil {
		t.Fatalf("Stat(%s): %v", keysPath, err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("private key mode = %o, want 600", info.Mode().Perm())
	}

	inspected, err := manager.InspectKeypair("production")
	if err != nil {
		t.Fatalf("InspectKeypair(): %v", err)
	}
	if inspected.PrivateKeyStatus != PrivateKeyValid {
		t.Errorf(
			"InspectKeypair() status = %q, want %q",
			inspected.PrivateKeyStatus,
			PrivateKeyValid,
		)
	}
}

// newLocalKeypairManager builds a manager over the default age cipher with a
// local key-file resolver and destination in dir.
func newLocalKeypairManager(t *testing.T, storePath, keysPath string) *Service {
	t.Helper()
	pkStore, err := pkfilestore.New(pkfilestore.Params{Path: keysPath})
	if err != nil {
		t.Fatalf("pkfilestore.New: %v", err)
	}
	pkService, err := privatekey.NewService(privatekey.ServiceParams{
		Repository: pkStore,
		LookupEnv:  func(string) (string, bool) { return "", false },
	})
	if err != nil {
		t.Fatalf("privatekey.NewService: %v", err)
	}
	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, storePath),
		Cipher:            newTestCipher(t),
		PrivateKeyService: pkService,
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return manager
}

// TestRotateKeypairReencryptsGroupWithNewIdentity verifies rotation replaces the
// public key, re-encrypts every value under the new key, and reports the change.
func TestRotateKeypairReencryptsGroupWithNewIdentity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "secrets.yaml")
	keysPath := filepath.Join(dir, "envx.keys")
	manager := newLocalKeypairManager(t, storePath, keysPath)

	if _, err := manager.GenerateKeypair("production"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	before, err := manager.InspectKeypair("production")
	if err != nil {
		t.Fatalf("InspectKeypair() before: %v", err)
	}
	if _, err := manager.SetSecret("production", "api_key", func() (string, error) {
		return "plain-api", nil
	}); err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	result, err := manager.RotateKeypair("production")
	if err != nil {
		t.Fatalf("RotateKeypair(): %v", err)
	}

	rotated := result.Keypair
	if rotated.PublicKey == before.PublicKey {
		t.Error("rotated public key is unchanged")
	}
	if rotated.PrivateKeyStatus != PrivateKeyValid {
		t.Errorf("PrivateKeyStatus = %q, want %q", rotated.PrivateKeyStatus, PrivateKeyValid)
	}
	got := referenceSet(result.Secrets)
	if _, ok := got[SecretReference{Group: "production", Key: "api_key"}]; !ok {
		t.Errorf("Secrets = %v, want production/api_key", result.Secrets)
	}

	// The stored value must decrypt with the freshly written private key.
	res, err := manager.GetSecret("production", "api_key")
	if err != nil {
		t.Fatalf("GetSecret() after rotation: %v", err)
	}
	if res.Value != "plain-api" {
		t.Errorf("GetSecret() = %q, want %q", res.Value, "plain-api")
	}
}

// TestRotateKeypairRejectsHigherPriorityKeyOrigin verifies rotation refuses the
// implicit local key file when the current key came from an environment variable.
func TestRotateKeypairRejectsHigherPriorityKeyOrigin(t *testing.T) {
	t.Parallel()

	selected := newTestCipher(t)
	pubKey, privKey, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	storePath := writeStore(t, "public-keys:\n  production: "+pubKey+"\n")
	keysPath := filepath.Join(filepath.Dir(storePath), "envx.keys")
	lookupEnv := func(name string) (string, bool) {
		if name == "ENVX_PRIVATE_KEY_PRODUCTION" {
			return privKey, true
		}
		return "", false
	}
	pkStore, err := pkfilestore.New(pkfilestore.Params{Path: keysPath})
	if err != nil {
		t.Fatalf("pkfilestore.New: %v", err)
	}
	pkService, err := privatekey.NewService(privatekey.ServiceParams{
		Repository: pkStore,
		LookupEnv:  lookupEnv,
	})
	if err != nil {
		t.Fatalf("privatekey.NewService: %v", err)
	}
	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, storePath),
		Cipher:            selected,
		PrivateKeyService: pkService,
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	_, err = manager.RotateKeypair("production")
	if err == nil {
		t.Fatal("RotateKeypair() succeeded for an environment-backed key")
	}
	if !strings.Contains(err.Error(), "explicit private-key destination") {
		t.Errorf("error = %q, want explicit-destination guidance", err)
	}
	if strings.Contains(err.Error(), privKey) {
		t.Fatal("rotation error contains private material")
	}
}

// TestRotateKeypairFailsWhenKeyUnavailable verifies rotation fails closed when
// the current private key cannot be resolved.
func TestRotateKeypairFailsWhenKeyUnavailable(t *testing.T) {
	t.Parallel()

	manager, _ := newBulkManager(
		t,
		"public-keys:\n  production: public-test-value\n",
		keypairTestService{
			err: fmt.Errorf("%w for group production", privatekey.ErrNotAvailable),
		},
	)

	_, err := manager.RotateKeypair("production")
	if err == nil {
		t.Fatal("RotateKeypair() succeeded without a private key")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("error = %q, want unavailable-key guidance", err)
	}
}

// TestRotateKeypairRequiresExistingIdentity verifies rotation refuses a group
// that has no public key to replace.
func TestRotateKeypairRequiresExistingIdentity(t *testing.T) {
	t.Parallel()

	manager, _ := newBulkManager(
		t, "public-keys: {}\n", fixedPrivateKeyResolver{value: "unused"},
	)

	_, err := manager.RotateKeypair("production")
	if err == nil {
		t.Fatal("RotateKeypair() succeeded for a missing identity")
	}
	if !strings.Contains(err.Error(), "no public key") {
		t.Errorf("error = %q, want missing-identity guidance", err)
	}
}
