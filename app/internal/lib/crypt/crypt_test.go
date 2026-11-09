package crypt_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/crypt"
)

func TestErrors(t *testing.T) {
	t.Parallel()

	if !errors.Is(crypt.ErrInvalidKey, crypt.ErrInvalidKey) {
		t.Errorf("expected ErrInvalidKey identity")
	}
	if !errors.Is(crypt.ErrDecrypt, crypt.ErrDecrypt) {
		t.Errorf("expected ErrDecrypt identity")
	}
}

func TestKeypair(t *testing.T) {
	t.Parallel()

	pair := crypt.Keypair{
		PublicKey:  "pub",
		PrivateKey: "priv",
	}
	if pair.PublicKey != "pub" || pair.PrivateKey != "priv" {
		t.Errorf("unexpected keypair fields: %+v", pair)
	}
}
