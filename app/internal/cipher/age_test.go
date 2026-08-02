package cipher

import (
	"bytes"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestAgeRoundTrip verifies key generation, native encryption, and decryption
// through the public Cipher interface.
func TestAgeRoundTrip(t *testing.T) {
	t.Parallel()

	selected, err := New(Age, AgeOptions{})
	if err != nil {
		t.Fatalf("New(age) error = %v", err)
	}
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}
	if !strings.HasPrefix(pair.PublicKey, "age-public-key:age1") {
		t.Fatalf("PublicKey = %q, want labeled age key", pair.PublicKey)
	}
	if !strings.HasPrefix(pair.PrivateKey, "age-private-key:AGE-SECRET-KEY-1") {
		t.Fatalf("PrivateKey = %q, want labeled age key", pair.PrivateKey)
	}

	plaintext := "line one\nline two"
	ciphertext, err := selected.Encrypt(plaintext, pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	decrypted, err := selected.Decrypt(ciphertext, pair.PrivateKey)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

// -------------------------------------------------------------------------------------

// TestAgeRejectsWrongKey ensures a ciphertext cannot be opened with another
// age identity.
func TestAgeRejectsWrongKey(t *testing.T) {
	t.Parallel()

	selected, err := New(Age, AgeOptions{})
	if err != nil {
		t.Fatalf("New(age) error = %v", err)
	}
	first, err := selected.Keypair()
	if err != nil {
		t.Fatalf("first Keypair() error = %v", err)
	}
	second, err := selected.Keypair()
	if err != nil {
		t.Fatalf("second Keypair() error = %v", err)
	}
	ciphertext, err := selected.Encrypt("secret", first.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if _, err := selected.Decrypt(ciphertext, second.PrivateKey); err == nil {
		t.Fatal("Decrypt() with the wrong key succeeded")
	}
}

// -------------------------------------------------------------------------------------

// TestAgeValidatesKeypair checks age key format and public/private matching.
func TestAgeValidatesKeypair(t *testing.T) {
	t.Parallel()

	selected, err := New(Age, AgeOptions{})
	if err != nil {
		t.Fatalf("New(age) error = %v", err)
	}
	first, err := selected.Keypair()
	if err != nil {
		t.Fatalf("first Keypair() error = %v", err)
	}
	second, err := selected.Keypair()
	if err != nil {
		t.Fatalf("second Keypair() error = %v", err)
	}

	tests := []struct {
		name       string
		publicKey  string
		privateKey string
	}{
		{
			name:       "malformed public",
			publicKey:  "invalid",
			privateKey: first.PrivateKey,
		},
		{
			name:       "malformed private",
			publicKey:  first.PublicKey,
			privateKey: "invalid",
		},
		{
			name:       "mismatched",
			publicKey:  first.PublicKey,
			privateKey: second.PrivateKey,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := selected.ValidateKeypair(test.publicKey, test.privateKey); err == nil {
				t.Fatal("ValidateKeypair() succeeded")
			}
		})
	}

	if err := selected.ValidateKeypair(first.PublicKey, first.PrivateKey); err != nil {
		t.Fatalf("ValidateKeypair() error = %v", err)
	}
}

// -------------------------------------------------------------------------------------

// TestAgeRejectsMalformedInputs keeps algorithm-specific parsing errors behind
// the Cipher boundary.
func TestAgeRejectsMalformedInputs(t *testing.T) {
	t.Parallel()

	selected, err := New(Age, AgeOptions{})
	if err != nil {
		t.Fatalf("New(age) error = %v", err)
	}
	if _, err := selected.Encrypt("secret", "not-a-public-key"); err == nil {
		t.Fatal("Encrypt() accepted a malformed public key")
	}
	if _, err := selected.Decrypt(
		[]byte("not-ciphertext"), "not-a-private-key",
	); err == nil {
		t.Fatal("Decrypt() accepted malformed ciphertext")
	}
}

// -------------------------------------------------------------------------------------

// TestAgeReturnsNativeCiphertext verifies age output is native protocol bytes,
// leaving single-line storage encoding to the secrets package.
func TestAgeReturnsNativeCiphertext(t *testing.T) {
	t.Parallel()

	selected, err := New(Age, AgeOptions{})
	if err != nil {
		t.Fatalf("New(age) error = %v", err)
	}
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}
	ciphertext, err := selected.Encrypt("secret", pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if bytes.HasPrefix(ciphertext, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		t.Fatal("Encrypt() returned armored ciphertext")
	}
	if !bytes.HasPrefix(ciphertext, []byte("age-encryption.org/v1\n")) {
		t.Fatal("Encrypt() did not return native age protocol bytes")
	}
	decrypted, err := selected.Decrypt(ciphertext, pair.PrivateKey)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != "secret" {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, "secret")
	}
}
