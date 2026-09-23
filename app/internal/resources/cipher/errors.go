package cipher

import "errors"

// ErrInvalidKey indicates malformed public or private key material.
var ErrInvalidKey = errors.New("invalid key material")

// ErrDecrypt indicates ciphertext that could not be decrypted with the given
// key, whether because the key does not match or the payload is corrupt.
var ErrDecrypt = errors.New("cannot decrypt ciphertext")
