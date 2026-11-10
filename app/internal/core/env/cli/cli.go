// Package cli provides CLI commands for environment resolution, inspection,
// modification, and execution.
package cli

import (
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/lib/cliflags"
)

// Flags local to environment commands.
var (
	// revealFlag decrypts referenced secret values instead of masking them.
	revealFlag = cliflags.Spec[bool]{
		Name:  "reveal",
		Usage: "decrypt secret references instead of masking them",
	}

	// absoluteFlag renders source paths absolutely instead of relative to envx.yaml.
	absoluteFlag = cliflags.Spec[bool]{
		Name:  "absolute",
		Usage: "show absolute source paths instead of paths relative to envx.yaml",
	}

	// outputFlag selects the rendering format for tabular commands.
	outputFlag = cliflags.Spec[string]{
		Name:  "output",
		Short: "o",
		Usage: "output format: table|json",
	}

	// ignoreErrorsFlag downgrades resolution failures to warnings.
	ignoreErrorsFlag = cliflags.Spec[bool]{
		Name:  "ignore-errors",
		Usage: "warn on unresolved values and omit them instead of aborting",
	}
)

// Factory defines the capabilities required for the environment commands.
type Factory interface {
	NewEnvService(configPath string) (*env.Service, error)
}
