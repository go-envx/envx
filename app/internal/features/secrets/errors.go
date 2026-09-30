package secrets

import "errors"

var (
	// ErrGroupNotFound indicates that no public key or secret exists for a group.
	ErrGroupNotFound = errors.New("secret group not found")
	// ErrSecretNotFound indicates that a requested key does not exist in a group.
	ErrSecretNotFound = errors.New("secret key not found")
	// ErrKeypairExists indicates an existing identity prevents overwriting.
	ErrKeypairExists = errors.New("keypair already exists")
	// ErrCiphertextMismatch indicates corrupt or tampered ciphertext.
	ErrCiphertextMismatch = errors.New("ciphertext verification failed")
	// ErrInvalidSecretKey indicates a malformed secret key identifier.
	ErrInvalidSecretKey = errors.New("invalid secret key")
)
