package secrets

import (
	"errors"
	"fmt"
)

var (
	// ErrConflict indicates a group changed after it was read, so a commit based
	// on the stale snapshot was rejected.
	ErrConflict = errors.New("secret group changed concurrently")
	// ErrNotReference indicates a value that is not a secret reference.
	ErrNotReference = errors.New("not a secret reference")
	// ErrGroupNotFound indicates that a group has no public key.
	ErrGroupNotFound = errors.New("secret group not found")
	// ErrSecretNotFound indicates that a requested key does not exist in a group.
	ErrSecretNotFound = errors.New("secret key not found")
	// ErrKeypairExists indicates an existing identity prevents overwriting.
	ErrKeypairExists = errors.New("keypair already exists")
	// ErrKeyNotAvailable indicates that no private key is available for a group.
	ErrKeyNotAvailable = errors.New("private key not available")
	// ErrInvalidKey indicates present but malformed private-key material.
	ErrInvalidKey = errors.New("invalid private key")
)

// notFound reports a reference with no stored value.
func notFound(ref Reference) error {
	return fmt.Errorf(
		"secret %q not found in group %q: %w", ref.Key, ref.Group, ErrSecretNotFound,
	)
}
