package runner

import "errors"

var (
	// ErrNoCommandSpecified indicates that an empty command slice was passed to Run.
	ErrNoCommandSpecified = errors.New("no command specified")
	// ErrProcessStartFailed indicates that the child executable could not be spawned.
	ErrProcessStartFailed = errors.New("failed to start process")
)
