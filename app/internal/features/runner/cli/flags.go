package cli

import "github.com/go-envx/envx/app/internal/shared/flags"

// ignoreErrorsFlag downgrades resolution failures to warnings.
var ignoreErrorsFlag = flags.Spec[bool]{
	Name:  "ignore-errors",
	Usage: "warn on unresolved values and omit them instead of aborting",
}
