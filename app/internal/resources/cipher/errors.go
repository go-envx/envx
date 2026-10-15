package cipher

import "errors"

var (
	// ErrInvalidKey indicates malformed public or private key material.
	ErrInvalidKey = errors.New("invalid key material")

	// ErrDecrypt indicates ciphertext that could not be decrypted with the given
	// key, whether because the key does not match or the payload is corrupt.
	ErrDecrypt = errors.New("cannot decrypt ciphertext")

	// ErrNilProvider indicates an attempt to register a nil Provider.
	ErrNilProvider = errors.New("cannot register nil cipher provider")

	// ErrEmptyAlgorithm indicates a Provider with an empty algorithm identifier.
	ErrEmptyAlgorithm = errors.New("cannot register cipher provider with empty algorithm")

	// ErrDuplicateAlgorithm indicates a provider algorithm is already registered.
	ErrDuplicateAlgorithm = errors.New("cipher algorithm already registered")

	// ErrUnknownAlgorithm indicates an unregistered algorithm identifier was requested.
	ErrUnknownAlgorithm = errors.New("unsupported cipher algorithm")
)
