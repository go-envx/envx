package cipher

import (
	"errors"
	"testing"
)

// TestDecryptWrongKeyIsErrDecrypt verifies a ciphertext opened with a
// well-formed but non-matching key classifies as ErrDecrypt.
func TestDecryptWrongKeyIsErrDecrypt(t *testing.T) {
	t.Parallel()

	for _, algorithm := range []Algorithm{Age, NaClBox} {
		selected, err := New(Params{Algorithm: algorithm})
		if err != nil {
			t.Fatalf("New(%s) error = %v", algorithm, err)
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

		_, err = selected.Decrypt(ciphertext, second.PrivateKey)
		if !errors.Is(err, ErrDecrypt) {
			t.Errorf("%s: Decrypt() error = %v, want ErrDecrypt", algorithm, err)
		}
	}
}

// TestDecryptMalformedKeyIsErrInvalidKey verifies a malformed private key
// classifies as ErrInvalidKey rather than a decrypt failure.
func TestDecryptMalformedKeyIsErrInvalidKey(t *testing.T) {
	t.Parallel()

	for _, algorithm := range []Algorithm{Age, NaClBox} {
		selected, err := New(Params{Algorithm: algorithm})
		if err != nil {
			t.Fatalf("New(%s) error = %v", algorithm, err)
		}
		pair, err := selected.Keypair()
		if err != nil {
			t.Fatalf("Keypair() error = %v", err)
		}
		ciphertext, err := selected.Encrypt("secret", pair.PublicKey)
		if err != nil {
			t.Fatalf("Encrypt() error = %v", err)
		}

		_, err = selected.Decrypt(ciphertext, "not-a-valid-private-key")
		if !errors.Is(err, ErrInvalidKey) {
			t.Errorf("%s: Decrypt() error = %v, want ErrInvalidKey", algorithm, err)
		}
	}
}
