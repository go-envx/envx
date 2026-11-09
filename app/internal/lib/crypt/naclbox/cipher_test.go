package naclbox_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/crypt"
	"github.com/go-envx/envx/app/internal/lib/crypt/naclbox"
)

func TestNaClBoxAlgorithm(t *testing.T) {
	t.Parallel()

	var c naclbox.Cipher
	if got := c.Algorithm(); got != naclbox.Algorithm {
		t.Fatalf("Algorithm() = %q, want %q", got, naclbox.Algorithm)
	}
}

func TestNaClBoxEncryptDecryptRoundTrip(t *testing.T) {
	t.Parallel()

	var c naclbox.Cipher

	pair, err := c.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}

	if err := c.ValidateKeypair(pair.PublicKey, pair.PrivateKey); err != nil {
		t.Fatalf("ValidateKeypair() error = %v", err)
	}

	plaintext := "hello sealed box"
	ciphertext, err := c.Encrypt(plaintext, pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if bytes.Equal(ciphertext, []byte(plaintext)) {
		t.Fatal("ciphertext equals plaintext")
	}

	decrypted, err := c.Decrypt(ciphertext, pair.PrivateKey)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestNaClBoxRejectsWrongKey(t *testing.T) {
	t.Parallel()

	var c naclbox.Cipher

	first, err := c.Keypair()
	if err != nil {
		t.Fatalf("first Keypair() error = %v", err)
	}
	second, err := c.Keypair()
	if err != nil {
		t.Fatalf("second Keypair() error = %v", err)
	}

	ciphertext, err := c.Encrypt("secret", first.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if _, err := c.Decrypt(ciphertext, second.PrivateKey); err == nil {
		t.Fatal("Decrypt() succeeded with wrong key")
	} else if !errors.Is(err, crypt.ErrDecrypt) {
		t.Fatalf("Decrypt() error = %v, want ErrDecrypt", err)
	}
}

func TestNaClBoxValidateKeypairErrors(t *testing.T) {
	t.Parallel()

	var c naclbox.Cipher

	pair, err := c.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}
	other, err := c.Keypair()
	if err != nil {
		t.Fatalf("other Keypair() error = %v", err)
	}

	tests := []struct {
		name       string
		publicKey  string
		privateKey string
	}{
		{
			name:       "missing public prefix",
			publicKey:  "invalid-pub",
			privateKey: pair.PrivateKey,
		},
		{
			name:       "malformed public key base64",
			publicKey:  "nacl-box-public-key:???",
			privateKey: pair.PrivateKey,
		},
		{
			name:       "short public key",
			publicKey:  "nacl-box-public-key:AQID",
			privateKey: pair.PrivateKey,
		},
		{
			name:       "missing private prefix",
			publicKey:  pair.PublicKey,
			privateKey: "invalid-priv",
		},
		{
			name:       "malformed private key base64",
			publicKey:  pair.PublicKey,
			privateKey: "nacl-box-private-key:???",
		},
		{
			name:       "short private key",
			publicKey:  pair.PublicKey,
			privateKey: "nacl-box-private-key:AQID",
		},
		{
			name:       "mismatched keys",
			publicKey:  pair.PublicKey,
			privateKey: other.PrivateKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := c.ValidateKeypair(tc.publicKey, tc.privateKey)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, crypt.ErrInvalidKey) {
				t.Fatalf("expected ErrInvalidKey, got %v", err)
			}
		})
	}
}

func TestNaClBoxEncryptInvalidKey(t *testing.T) {
	t.Parallel()

	var c naclbox.Cipher

	if _, err := c.Encrypt("plaintext", "invalid-pub"); !errors.Is(
		err, crypt.ErrInvalidKey,
	) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}

	if _, err := c.Encrypt("plaintext", "nacl-box-public-key:short"); !errors.Is(
		err, crypt.ErrInvalidKey,
	) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestNaClBoxDecryptInvalidKey(t *testing.T) {
	t.Parallel()

	var c naclbox.Cipher

	if _, err := c.Decrypt([]byte("data"), "invalid-priv"); !errors.Is(
		err, crypt.ErrInvalidKey,
	) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}

	if _, err := c.Decrypt([]byte("data"), "nacl-box-private-key:short"); !errors.Is(
		err, crypt.ErrInvalidKey,
	) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}
