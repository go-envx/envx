package crypt

import "errors"

var (
	// ErrInvalidKey indicates malformed public or private key material.
	ErrInvalidKey = errors.New("invalid key material")

	// ErrDecrypt indicates ciphertext that could not be decrypted with the given
	// key, whether because the key does not match or the payload is corrupt.
	ErrDecrypt = errors.New("cannot decrypt ciphertext")
)

// Keypair represents an opaque public/private key pair produced by a cipher.
// PrivateKey is transient material and must not appear in status or mutation results.
type Keypair struct {
	// PublicKey is the key used to encrypt values.
	PublicKey string
	// PrivateKey is the key used to decrypt values.
	PrivateKey string
}
