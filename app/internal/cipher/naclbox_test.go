package cipher

import (
	"bytes"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestNaClBoxRoundTrip verifies key generation, anonymous encryption, and
// decryption through the public Cipher interface.
func TestNaClBoxRoundTrip(t *testing.T) {
	t.Parallel()

	selected, err := New(NaClBox, NaClBoxOptions{})
	if err != nil {
		t.Fatalf("New(NaClBox) error = %v", err)
	}
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}
	if !strings.HasPrefix(pair.PublicKey, "nacl-box-public-key:") {
		t.Fatalf("PublicKey = %q, missing public key marker", pair.PublicKey)
	}
	if !strings.HasPrefix(pair.PrivateKey, "nacl-box-private-key:") {
		t.Fatalf("PrivateKey = %q, missing private key marker", pair.PrivateKey)
	}

	plaintext := "line one\nline two"
	ciphertext, err := selected.Encrypt(plaintext, pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if bytes.Equal(ciphertext, []byte(plaintext)) {
		t.Fatal("Encrypt() returned plaintext")
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

// TestNaClBoxRejectsWrongKey ensures a ciphertext cannot be opened with
// another NaCl Box identity.
func TestNaClBoxRejectsWrongKey(t *testing.T) {
	t.Parallel()

	selected, err := New(NaClBox, NaClBoxOptions{})
	if err != nil {
		t.Fatalf("New(NaClBox) error = %v", err)
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

// TestNaClBoxValidatesKeypair checks key format and public/private matching.
func TestNaClBoxValidatesKeypair(t *testing.T) {
	t.Parallel()

	selected, err := New(NaClBox, NaClBoxOptions{})
	if err != nil {
		t.Fatalf("New(NaClBox) error = %v", err)
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

// TestNaClBoxRejectsTampering ensures authenticated ciphertext modification is
// rejected.
func TestNaClBoxRejectsTampering(t *testing.T) {
	t.Parallel()

	selected, err := New(NaClBox, NaClBoxOptions{})
	if err != nil {
		t.Fatalf("New(NaClBox) error = %v", err)
	}
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}
	ciphertext, err := selected.Encrypt("secret", pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 1

	if _, err := selected.Decrypt(tampered, pair.PrivateKey); err == nil {
		t.Fatal("Decrypt() accepted tampered ciphertext")
	}
}

// -------------------------------------------------------------------------------------

// TestNaClBoxRejectsMalformedKeys ensures key markers and lengths are checked
// before cryptographic operations.
func TestNaClBoxRejectsMalformedKeys(t *testing.T) {
	t.Parallel()

	selected, err := New(NaClBox, NaClBoxOptions{})
	if err != nil {
		t.Fatalf("New(NaClBox) error = %v", err)
	}
	if _, err := selected.Encrypt("secret", "not-a-public-key"); err == nil {
		t.Fatal("Encrypt() accepted a malformed public key")
	}
	if _, err := selected.Decrypt(
		[]byte("not-ciphertext"), "not-a-private-key",
	); err == nil {
		t.Fatal("Decrypt() accepted malformed private key material")
	}
}
