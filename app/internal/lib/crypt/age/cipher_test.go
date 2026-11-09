package age_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/crypt"
	"github.com/go-envx/envx/app/internal/lib/crypt/age"
)

func TestAgeAlgorithm(t *testing.T) {
	t.Parallel()

	var c age.Cipher
	if got := c.Algorithm(); got != age.Algorithm {
		t.Fatalf("Algorithm() = %q, want %q", got, age.Algorithm)
	}
}

func TestAgeEncryptDecryptRoundTrip(t *testing.T) {
	t.Parallel()

	var c age.Cipher

	pair, err := c.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}

	if err := c.ValidateKeypair(pair.PublicKey, pair.PrivateKey); err != nil {
		t.Fatalf("ValidateKeypair() error = %v", err)
	}

	plaintext := "hello secret world"
	ciphertext, err := c.Encrypt(plaintext, pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := c.Decrypt(ciphertext, pair.PrivateKey)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestAgeRejectsWrongKey(t *testing.T) {
	t.Parallel()

	var c age.Cipher

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

func TestAgeReturnsNativeCiphertext(t *testing.T) {
	t.Parallel()

	var c age.Cipher

	pair, err := c.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}

	ciphertext, err := c.Encrypt("secret", pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if bytes.HasPrefix(ciphertext, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		t.Fatalf("Encrypt() produced armored ciphertext: %s", ciphertext)
	}
}

func TestAgeValidateKeypairErrors(t *testing.T) {
	t.Parallel()

	var c age.Cipher

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
			name:       "corrupted public key",
			publicKey:  "age-public-key:corrupted",
			privateKey: pair.PrivateKey,
		},
		{
			name:       "missing private prefix",
			publicKey:  pair.PublicKey,
			privateKey: "invalid-priv",
		},
		{
			name:       "corrupted private key",
			publicKey:  pair.PublicKey,
			privateKey: "age-private-key:corrupted",
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

func TestAgeEncryptInvalidKey(t *testing.T) {
	t.Parallel()

	var c age.Cipher

	if _, err := c.Encrypt("plaintext", "invalid-pub"); !errors.Is(
		err, crypt.ErrInvalidKey,
	) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}

	if _, err := c.Encrypt("plaintext", "age-public-key:corrupt"); !errors.Is(
		err, crypt.ErrInvalidKey,
	) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestAgeDecryptInvalidKey(t *testing.T) {
	t.Parallel()

	var c age.Cipher

	if _, err := c.Decrypt([]byte("data"), "invalid-priv"); !errors.Is(
		err, crypt.ErrInvalidKey,
	) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}

	if _, err := c.Decrypt([]byte("data"), "age-private-key:corrupt"); !errors.Is(
		err, crypt.ErrInvalidKey,
	) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}
