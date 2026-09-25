package privatekey

import "errors"

var (
	// ErrNotAvailable indicates that no private key is available for a group.
	ErrNotAvailable = errors.New("private key not available")
	// ErrInvalidKey indicates a present but malformed private key.
	ErrInvalidKey = errors.New("invalid private key")
	// ErrEmptyGroup indicates that a group name is empty or only whitespace.
	ErrEmptyGroup = errors.New("private-key group is empty")
	// ErrInvalidGroup indicates a group name containing whitespace, '=', or line breaks.
	ErrInvalidGroup = errors.New("invalid private-key group")
	// ErrEmptyKey indicates that a private key is empty.
	ErrEmptyKey = errors.New("private key is empty")
	// ErrKeyHasLineBreak indicates that a private key contains a line break.
	ErrKeyHasLineBreak = errors.New("private key contains a line break")
)
