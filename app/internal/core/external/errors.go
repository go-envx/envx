package external

import "errors"

var (
	// ErrNotReference reports a value that does not use an external scheme.
	ErrNotReference = errors.New("not an external reference")
	// ErrInvalidReference reports a malformed external reference.
	ErrInvalidReference = errors.New("invalid external reference")
	// ErrProviderUnavailable reports a provider whose CLI or credentials are
	// missing, or that timed out earlier in the operation.
	ErrProviderUnavailable = errors.New("external provider unavailable")
	// ErrReferenceUnresolved reports a provider that returned no value.
	ErrReferenceUnresolved = errors.New("external reference unresolved")
)
