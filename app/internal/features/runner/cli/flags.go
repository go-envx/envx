package cli

import "github.com/go-envx/envx/app/internal/utils/cliflags"

// Flags local to run command.
var (
	// IgnoreErrors downgrades resolution failures to warnings.
	IgnoreErrors = cliflags.FlagSpec{
		Name:  "ignore-errors",
		Usage: "warn on unresolved values and omit them instead of aborting",
	}
)
