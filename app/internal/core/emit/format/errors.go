package format

import "errors"

var (
	// ErrUnknownTarget indicates an unsupported emit target.
	ErrUnknownTarget = errors.New("unknown emit target")
	// ErrInvalidOption indicates an invalid option or target/option combination.
	ErrInvalidOption = errors.New("invalid emit option")
	// ErrMissingNameBase indicates a Kubernetes target without a resource name.
	ErrMissingNameBase = errors.New("kubernetes target requires a name base")
)
