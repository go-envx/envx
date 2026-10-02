package emit

import "errors"

var (
	// ErrUnknownTarget indicates that an unsupported serialization target was requested.
	ErrUnknownTarget = errors.New("unknown emit target")
	// ErrMissingNameBase indicates that a Kubernetes target lacks a resource name base.
	ErrMissingNameBase = errors.New("kubernetes target requires a name base")
	// ErrNoSliceSelected indicates that neither secrets nor config was selected.
	ErrNoSliceSelected = errors.New("no slice selected: select secrets, config, or both")
)
