package secrets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/privatekey"
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
)

// -------------------------------------------------------------------------------------

// keypairTestCipher is a deterministic cipher double for keypair workflow tests.
type keypairTestCipher struct {
	// pair is returned by Keypair.
	pair cipher.Keypair
	// validPrivate is the only private key ValidateKeypair accepts.
	validPrivate string
}

// -------------------------------------------------------------------------------------

// Algorithm identifies the algorithm metadata used by the test cipher.
func (keypairTestCipher) Algorithm() cipher.Algorithm {
	return cipher.Age
}

// -------------------------------------------------------------------------------------

// Keypair returns the deterministic test keypair.
func (c keypairTestCipher) Keypair() (cipher.Keypair, error) {
	return c.pair, nil
}

// -------------------------------------------------------------------------------------

// ValidateKeypair validates the deterministic test keypair.
func (c keypairTestCipher) ValidateKeypair(publicKey, privateKey string) error {
	if publicKey != c.pair.PublicKey || privateKey != c.validPrivate {
		return errors.New("test keypair mismatch")
	}
	return nil
}

// -------------------------------------------------------------------------------------

// Encrypt is unused by keypair workflow tests.
func (keypairTestCipher) Encrypt(string, string) ([]byte, error) {
	return nil, errors.New("test cipher encryption is unused")
}

// -------------------------------------------------------------------------------------

// Decrypt is unused by keypair workflow tests.
func (keypairTestCipher) Decrypt([]byte, string) (string, error) {
	return "", errors.New("test cipher decryption is unused")
}

// -------------------------------------------------------------------------------------

// keypairTestDestination records the write point for a generated private key.
type keypairTestDestination struct {
	// write records the destination call and may inspect the store before commit.
	write func(group, privateKey string) error
}

// -------------------------------------------------------------------------------------

// Write records a generated private-key handoff without storing the key.
func (d keypairTestDestination) Write(group, privateKey string) error {
	return d.write(group, privateKey)
}

// -------------------------------------------------------------------------------------

// keypairTestResolver returns one configured private-key resolver result.
type keypairTestResolver struct {
	// key is returned by Resolve.
	key privatekey.PrivateKey
	// err is returned by Resolve.
	err error
}

// -------------------------------------------------------------------------------------

// Resolve returns the configured test source result.
func (r keypairTestResolver) Resolve(string) (privatekey.PrivateKey, error) {
	return r.key, r.err
}

// -------------------------------------------------------------------------------------

// TestGenerateKeypairCommitsPublicStateAfterPrivateHandoff verifies the safe
// write order and that the result does not carry private material.
func TestGenerateKeypairCommitsPublicStateAfterPrivateHandoff(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "secrets.yaml")
	keysPath := filepath.Join(dir, "envx.keys")
	const privateValue = "private-test-value"
	cipherDouble := keypairTestCipher{
		pair: cipher.Keypair{
			PublicKey:  "public-test-value",
			PrivateKey: privateValue,
		},
		validPrivate: privateValue,
	}
	destination := keypairTestDestination{
		write: func(group, privateKey string) error {
			if group != "production" || privateKey != privateValue {
				t.Fatalf("destination received (%q, %q)", group, privateKey)
			}
			document, err := store.Open(storePath)
			if err != nil {
				return err
			}
			if _, exists := document.PublicKey(group); exists {
				return errors.New("public key was committed before private handoff")
			}
			return nil
		},
	}

	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              keysPath,
		Cipher:                cipherDouble,
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: destination,
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	metadata, err := manager.GenerateKeypair("production")
	if err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if metadata.PrivateKeyStatus != PrivateKeyValid {
		t.Errorf("PrivateKeyStatus = %q, want %q", metadata.PrivateKeyStatus, PrivateKeyValid)
	}
	if strings.Contains(fmt.Sprintf("%+v", metadata), privateValue) {
		t.Fatal("keypair metadata contains private material")
	}

	document, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("Open() after generation: %v", err)
	}
	got, exists := document.PublicKey("PRODUCTION")
	if !exists || got != "public-test-value" {
		t.Errorf("PublicKey() = (%q, %v)", got, exists)
	}
}

// -------------------------------------------------------------------------------------

// TestGenerateKeypairRefusesExistingIdentity verifies generation never replaces
// a group's existing public identity.
func TestGenerateKeypairRefusesExistingIdentity(t *testing.T) {
	t.Parallel()

	storePath := writeStore(t, "public_keys:\n  production: existing-public\n")
	called := false
	manager, err := New(Params{
		SecretsPath: storePath,
		KeysPath:    filepath.Join(filepath.Dir(storePath), "envx.keys"),
		Cipher: keypairTestCipher{
			pair:         cipher.Keypair{PublicKey: "new-public", PrivateKey: "new-private"},
			validPrivate: "new-private",
		},
		PrivateKeyResolver: newPrivateKeyTestResolver(),
		PrivateKeyDestination: keypairTestDestination{
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

// -------------------------------------------------------------------------------------

// TestInspectKeypairStatuses verifies unavailable, valid, and invalid private
// key states without returning private material.
func TestInspectKeypairStatuses(t *testing.T) {
	t.Parallel()

	storePath := writeStore(t, "public_keys:\n  production: public-test-value\n")
	cipherDouble := keypairTestCipher{
		pair: cipher.Keypair{
			PublicKey:  "public-test-value",
			PrivateKey: "private-test-value",
		},
		validPrivate: "private-test-value",
	}
	tests := []struct {
		name     string
		resolver privatekey.Resolver
		want     PrivateKeyStatus
	}{
		{
			name: "unavailable",
			resolver: keypairTestResolver{
				err: fmt.Errorf("%w for group production", privatekey.ErrNotAvailable),
			},
			want: PrivateKeyNotAvailable,
		},
		{
			name: "valid",
			resolver: keypairTestResolver{
				key: privatekey.PrivateKey{Value: "private-test-value", Origin: "test"},
			},
			want: PrivateKeyValid,
		},
		{
			name: "invalid",
			resolver: keypairTestResolver{
				key: privatekey.PrivateKey{Value: "wrong-private-value", Origin: "test"},
			},
			want: PrivateKeyInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			manager, err := New(Params{
				SecretsPath:           storePath,
				KeysPath:              filepath.Join(t.TempDir(), "envx.keys"),
				Cipher:                cipherDouble,
				PrivateKeyResolver:    tt.resolver,
				PrivateKeyDestination: newPrivateKeyTestDestination(),
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

// -------------------------------------------------------------------------------------

// TestGenerateKeypairReportsRecoverableStoreFailure verifies a private-key
// handoff failure after validation never exposes private material in its error.
func TestGenerateKeypairReportsRecoverableStoreFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "missing", "secrets.yaml")
	keysPath := filepath.Join(dir, "envx.keys")
	const privateValue = "private-test-value"
	manager, err := New(Params{
		SecretsPath: storePath,
		KeysPath:    keysPath,
		Cipher: keypairTestCipher{
			pair: cipher.Keypair{
				PublicKey:  "public-test-value",
				PrivateKey: privateValue,
			},
			validPrivate: privateValue,
		},
		PrivateKeyResolver: newPrivateKeyTestResolver(),
		PrivateKeyDestination: keypairTestDestination{
			write: func(string, string) error { return nil },
		},
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	_, err = manager.GenerateKeypair("production")
	if err == nil {
		t.Fatal("GenerateKeypair() succeeded with an unwritable store")
	}
	if !strings.Contains(err.Error(), "retry") {
		t.Errorf("error = %q, want recovery guidance", err)
	}
	if strings.Contains(err.Error(), privateValue) {
		t.Fatal("generation error contains private material")
	}
}

// -------------------------------------------------------------------------------------

// TestEnsureGitIgnoredSkipsWhenGitIsUnavailable verifies no ignore file is
// created when the Git executable cannot be found.
func TestEnsureGitIgnoredSkipsWhenGitIsUnavailable(t *testing.T) {
	gitlessPath := t.TempDir()
	t.Setenv("PATH", gitlessPath)

	keysPath := filepath.Join(t.TempDir(), "nested", "envx.keys")
	if err := ensureGitIgnored(keysPath); err != nil {
		t.Fatalf("ensureGitIgnored(): %v", err)
	}
	ignorePath := filepath.Join(filepath.Dir(keysPath), ".gitignore")
	if _, err := os.Stat(ignorePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".gitignore stat error = %v, want file not to exist", err)
	}
}

// -------------------------------------------------------------------------------------

// TestGenerateDefaultKeypairRoundTrip verifies the default age cipher, local
// key-file destination, private permissions, and local Git ignore protection.
func TestGenerateDefaultKeypairRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "secrets.yaml")
	keysPath := filepath.Join(dir, "envx.keys")
	manager, err := New(Params{
		SecretsPath: storePath,
		KeysPath:    keysPath,
		Cipher:      newTestCipher(t),
		PrivateKeyResolver: privatekey.NewResolver(privatekey.ResolverOptions{
			KeysPath: keysPath,
		}),
		PrivateKeyDestination: privatekey.NewFileDestination(keysPath),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	metadata, err := manager.GenerateKeypair("production")
	if err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if metadata.PrivateKeyStatus != PrivateKeyValid {
		t.Errorf("PrivateKeyStatus = %q, want %q", metadata.PrivateKeyStatus, PrivateKeyValid)
	}

	info, err := os.Stat(keysPath)
	if err != nil {
		t.Fatalf("Stat(%s): %v", keysPath, err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("private key mode = %o, want 600", info.Mode().Perm())
	}
	ignore, err := os.ReadFile( //nolint:gosec // test-local path.
		filepath.Join(dir, ".gitignore"),
	)
	if err != nil {
		t.Fatalf("ReadFile(.gitignore): %v", err)
	}
	if !strings.Contains(string(ignore), "envx.keys") {
		t.Errorf(".gitignore = %q, want envx.keys rule", ignore)
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
