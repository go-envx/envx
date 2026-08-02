package cipher

import (
	"fmt"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestNew selects registered algorithms and rejects unsupported option types.
func TestNew(t *testing.T) {
	t.Parallel()

	selected, err := New(Age, AgeOptions{})
	if err != nil {
		t.Fatalf("New(age) error = %v", err)
	}
	if selected == nil {
		t.Fatal("New(age) returned a nil cipher")
	}
	selected, err = New(NaClBox, NaClBoxOptions{})
	if err != nil {
		t.Fatalf("New(NaClBox) error = %v", err)
	}
	if selected == nil {
		t.Fatal("New(NaClBox) returned a nil cipher")
	}

	if _, err := New(Age, NaClBoxOptions{}); err == nil {
		t.Fatal("New(age) accepted NaClBoxOptions")
	}

	if _, err := New(Algorithm("unknown"), AgeOptions{}); err == nil {
		t.Fatal("New(unknown) succeeded")
	}
}

// -------------------------------------------------------------------------------------

// TestCipherInterfaceAcceptsAnotherImplementation verifies a non-age
// implementation can satisfy the algorithm-neutral contract.
func TestCipherInterfaceAcceptsAnotherImplementation(t *testing.T) {
	t.Parallel()

	var selected Cipher = alternateCipher{}
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("alternate Keypair() error = %v", err)
	}
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

// -------------------------------------------------------------------------------------

// alternateCipher is a test-only implementation that verifies callers depend
// on the Cipher contract rather than age-specific details.
type alternateCipher struct{}

// -------------------------------------------------------------------------------------

// Keypair returns representative opaque key strings for the test cipher.
func (alternateCipher) Keypair() (Keypair, error) {
	return Keypair{PublicKey: "alternate-public", PrivateKey: "alternate-private"}, nil
}

// -------------------------------------------------------------------------------------

// ValidateKeypair accepts the fixed key material used by the test cipher.
func (alternateCipher) ValidateKeypair(publicKey, privateKey string) error {
	if publicKey != "alternate-public" || privateKey != "alternate-private" {
		return fmt.Errorf("invalid alternate keypair")
	}
	return nil
}

// -------------------------------------------------------------------------------------

// Encrypt returns a marker-prefixed value for the test cipher.
func (alternateCipher) Encrypt(plaintext, _ string) ([]byte, error) {
	return []byte("alternate:" + plaintext), nil
}

// -------------------------------------------------------------------------------------

// Decrypt removes the test cipher marker.
func (alternateCipher) Decrypt(ciphertext []byte, _ string) (string, error) {
	return strings.TrimPrefix(string(ciphertext), "alternate:"), nil
}
