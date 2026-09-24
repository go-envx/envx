// Package flags defines presentation flags shared across multiple CLI command slices.
package flags

import "github.com/go-envx/envx/app/internal/utils/cliflags"

// Shared presentation flags.
var (
	// Output selects the rendering format for tabular commands.
	Output = cliflags.FlagSpec{
		Name:  "output",
		Short: "o",
		Usage: "output format: table|json",
	}

	// Verbose prints additional detail in command output.
	Verbose = cliflags.FlagSpec{
		Name:  "verbose",
		Short: "v",
		Usage: "print detailed output",
	}

	// Reveal decrypts referenced secret values instead of masking them.
	Reveal = cliflags.FlagSpec{
		Name:  "reveal",
		Usage: "decrypt secret references instead of masking them",
	}

	// Absolute renders source paths absolutely instead of relative to envx.yaml.
	Absolute = cliflags.FlagSpec{
		Name:  "absolute",
		Usage: "show absolute source paths instead of paths relative to envx.yaml",
	}
)
