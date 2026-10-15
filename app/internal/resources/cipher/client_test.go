package cipher_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/value"
)

// TestProviderInterfaceAcceptsAnotherImplementation verifies an external
// implementation can satisfy the algorithm-neutral contract.
func TestProviderInterfaceAcceptsAnotherImplementation(t *testing.T) {
	t.Parallel()

	var selected cipher.Provider = alternateCipher{name: "alternate"}
	pair := newKeypair(t, selected)
	if pair.PublicKey == "" || pair.PrivateKey == "" {
		t.Fatal("alternate Keypair() returned incomplete key material")
	}
	if err := selected.ValidateKeypair(pair.PublicKey, pair.PrivateKey); err != nil {
		t.Fatalf("alternate ValidateKeypair() error = %v", err)
	}
	ciphertext, err := selected.Encrypt("secret", pair.PublicKey)
	if err != nil {
		t.Fatalf("alternate Encrypt() error = %v", err)
	}
	plaintext, err := selected.Decrypt(ciphertext, pair.PrivateKey)
	if err != nil {
		t.Fatalf("alternate Decrypt() error = %v", err)
	}
	if plaintext != "secret" {
		t.Fatalf("alternate Decrypt() = %q, want %q", plaintext, "secret")
	}
}

// TestClientRegistryOperations verifies Register, For, and All.
func TestClientRegistryOperations(t *testing.T) {
	t.Parallel()

	provider1 := alternateCipher{name: "test-1"}
	provider2 := alternateCipher{name: "test-2"}

	client, err := cipher.New(cipher.Params{
		Providers: []cipher.Provider{provider1},
	})
	if err != nil {
		t.Fatalf("cipher.New() error = %v", err)
	}

	// For existing
	p, err := client.For("test-1")
	if err != nil {
		t.Fatalf("client.For(\"test-1\") error = %v", err)
	}
	if p.Algorithm() != "test-1" {
		t.Fatalf("got algorithm %q, want %q", p.Algorithm(), "test-1")
	}

	// For missing
	_, err = client.For("nonexistent")
	if !errors.Is(err, cipher.ErrUnknownAlgorithm) {
		t.Fatalf("client.For(\"nonexistent\") err = %v, want ErrUnknownAlgorithm", err)
	}

	// Register new
	if err := client.Register(provider2); err != nil {
		t.Fatalf("client.Register(provider2) error = %v", err)
	}

	// Register duplicate
	if err := client.Register(provider2); !errors.Is(err, cipher.ErrDuplicateAlgorithm) {
		t.Fatalf("client.Register duplicate err = %v, want ErrDuplicateAlgorithm", err)
	}

	// Register nil
	if err := client.Register(nil); !errors.Is(err, cipher.ErrNilProvider) {
		t.Fatalf("client.Register(nil) err = %v, want ErrNilProvider", err)
	}

	// Register empty name
	emptyProvider := alternateCipher{name: ""}
	if err := client.Register(emptyProvider); !errors.Is(err, cipher.ErrEmptyAlgorithm) {
		t.Fatalf("client.Register empty name err = %v, want ErrEmptyAlgorithm", err)
	}

	// All
	all := client.All()
	if len(all) != 2 {
		t.Fatalf("len(client.All()) = %d, want 2", len(all))
	}
	if all["test-1"] == nil || all["test-2"] == nil {
		t.Fatalf("client.All() missing registered providers: %v", all)
	}
}

// newKeypair generates a keypair from c, failing the test on error.
func newKeypair(t *testing.T, c cipher.Provider) value.Keypair {
	t.Helper()
	pair, err := c.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}
	return pair
}

// alternateCipher is a test-only implementation that verifies callers depend
// on the Provider contract rather than algorithm-specific details.
type alternateCipher struct {
	name string
}

// Algorithm identifies the algorithm metadata used by the test cipher.
func (a alternateCipher) Algorithm() string {
	return a.name
}

// Keypair returns representative opaque key strings for the test cipher.
func (alternateCipher) Keypair() (value.Keypair, error) {
	return value.Keypair{
		PublicKey:  "alternate-public",
		PrivateKey: "alternate-private",
	}, nil
}

// ValidateKeypair accepts the fixed key material used by the test cipher.
func (alternateCipher) ValidateKeypair(publicKey, privateKey string) error {
	if publicKey != "alternate-public" || privateKey != "alternate-private" {
		return fmt.Errorf("invalid alternate keypair")
	}
	return nil
}

// Encrypt returns a marker-prefixed value for the test cipher.
func (alternateCipher) Encrypt(plaintext, _ string) ([]byte, error) {
	return []byte("alternate:" + plaintext), nil
}

// Decrypt removes the test cipher marker.
func (alternateCipher) Decrypt(ciphertext []byte, _ string) (string, error) {
	return strings.TrimPrefix(string(ciphertext), "alternate:"), nil
}
