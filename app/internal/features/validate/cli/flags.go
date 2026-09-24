package cli

import "github.com/go-envx/envx/app/internal/shared/flags"

// Flags local to validate command.
var (
	// Strict fails validation on warnings as well as errors.
	Strict = flags.Spec[bool]{
		Name:  "strict",
		Usage: "fail on warnings as well as errors",
	}
)
