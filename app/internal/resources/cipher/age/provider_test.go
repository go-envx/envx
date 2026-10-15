package age_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/resources/cipher/age"
)

func TestAgeEncryptDecryptRoundTrip(t *testing.T) {
	t.Parallel()

	client, err := age.New(age.Params{})
	if err != nil {
		t.Fatalf("age.New() error = %v", err)
	}

	pair, err := client.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}

	if err := client.ValidateKeypair(pair.PublicKey, pair.PrivateKey); err != nil {
		t.Fatalf("ValidateKeypair() error = %v", err)
	}

	plaintext := "hello secret world"
	ciphertext, err := client.Encrypt(plaintext, pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := client.Decrypt(ciphertext, pair.PrivateKey)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestAgeRejectsWrongKey(t *testing.T) {
	t.Parallel()

	client, err := age.New(age.Params{})
	if err != nil {
		t.Fatalf("age.New() error = %v", err)
	}

	first, err := client.Keypair()
	if err != nil {
		t.Fatalf("first Keypair() error = %v", err)
	}
	second, err := client.Keypair()
	if err != nil {
		t.Fatalf("second Keypair() error = %v", err)
	}

	ciphertext, err := client.Encrypt("secret", first.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if _, err := client.Decrypt(ciphertext, second.PrivateKey); err == nil {
		t.Fatal("Decrypt() succeeded with wrong key")
	} else if !errors.Is(err, cipher.ErrDecrypt) {
		t.Fatalf("Decrypt() error = %v, want ErrDecrypt", err)
	}
}

func TestAgeReturnsNativeCiphertext(t *testing.T) {
	t.Parallel()

	client, err := age.New(age.Params{})
	if err != nil {
		t.Fatalf("age.New() error = %v", err)
	}

	pair, err := client.Keypair()
	if err != nil {
		t.Fatalf("Keypair() error = %v", err)
	}

	ciphertext, err := client.Encrypt("secret", pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if bytes.HasPrefix(ciphertext, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		t.Fatalf("Encrypt() produced armored ciphertext: %s", ciphertext)
	}
}
