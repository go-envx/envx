package cli

import "github.com/go-envx/envx/app/internal/shared/flags"

// Flags local to run command.
var (
	// IgnoreErrors downgrades resolution failures to warnings.
	IgnoreErrors = flags.Spec[bool]{
		Name:  "ignore-errors",
		Usage: "warn on unresolved values and omit them instead of aborting",
	}
)
