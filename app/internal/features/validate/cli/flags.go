package cli

import "github.com/go-envx/envx/app/internal/utils/cliflags"

// Flags local to validate command.
var (
	// Strict fails validation on warnings as well as errors.
	Strict = cliflags.FlagSpec{
		Name:  "strict",
		Usage: "fail on warnings as well as errors",
	}
)
