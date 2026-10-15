package cipher_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/resources/cipher/age"
	"github.com/go-envx/envx/app/internal/resources/cipher/naclbox"
)

// TestDecryptWrongKeyIsErrDecrypt verifies a ciphertext opened with a
// well-formed but non-matching key classifies as ErrDecrypt.
func TestDecryptWrongKeyIsErrDecrypt(t *testing.T) {
	t.Parallel()

	ageClient, err := age.New(age.Params{})
	if err != nil {
		t.Fatalf("age.New() error = %v", err)
	}
	naclClient, err := naclbox.New(naclbox.Params{})
	if err != nil {
		t.Fatalf("naclbox.New() error = %v", err)
	}

	for _, client := range []cipher.Provider{ageClient, naclClient} {
		first := newKeypair(t, client)
		second := newKeypair(t, client)
		ciphertext, err := client.Encrypt("secret", first.PublicKey)
		if err != nil {
			t.Fatalf("Encrypt() error = %v", err)
		}

		_, err = client.Decrypt(ciphertext, second.PrivateKey)
		if !errors.Is(err, cipher.ErrDecrypt) {
			t.Errorf("%s: Decrypt() error = %v, want ErrDecrypt", client.Algorithm(), err)
		}
	}
}

// TestDecryptMalformedKeyIsErrInvalidKey verifies a malformed private key
// classifies as ErrInvalidKey rather than a decrypt failure.
func TestDecryptMalformedKeyIsErrInvalidKey(t *testing.T) {
	t.Parallel()

	ageClient, err := age.New(age.Params{})
	if err != nil {
		t.Fatalf("age.New() error = %v", err)
	}
	naclClient, err := naclbox.New(naclbox.Params{})
	if err != nil {
		t.Fatalf("naclbox.New() error = %v", err)
	}

	for _, client := range []cipher.Provider{ageClient, naclClient} {
		pair := newKeypair(t, client)
		ciphertext, err := client.Encrypt("secret", pair.PublicKey)
		if err != nil {
			t.Fatalf("Encrypt() error = %v", err)
		}

		_, err = client.Decrypt(ciphertext, "not-a-valid-private-key")
		if !errors.Is(err, cipher.ErrInvalidKey) {
			t.Errorf("%s: Decrypt() error = %v, want ErrInvalidKey", client.Algorithm(), err)
		}
	}
}
