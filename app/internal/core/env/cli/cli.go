// Package cli provides CLI commands for environment resolution, inspection,
// modification, emission, and execution.
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

	// targetFlag selects the output shape the emit command renders to.
	targetFlag = cliflags.Spec[string]{
		Name:  "target",
		Short: "t",
		Usage: "output shape: k8s|k8s-bundle|json|dotenv",
	}

	// onlyFlag restricts the emitted output to one slice of the environment:
	// "secrets" for the secret-derived values or "config" for the plain ones.
	onlyFlag = cliflags.Spec[string]{
		Name:  "only",
		Usage: "restrict output to one slice: secrets|config (default: all values)",
	}

	// nameFlag overrides the base for k8s resource names.
	nameFlag = cliflags.Spec[string]{
		Name:  "name",
		Short: "n",
		Usage: "base for k8s resource names (default: project name; k8s targets only)",
	}

	// keyFlag overrides the k8s-bundle data key and mounted filename.
	keyFlag = cliflags.Spec[string]{
		Name:  "key",
		Usage: "k8s-bundle data key / filename (default: config.json or secrets.json)",
	}

	// emitOutputFlag writes the rendered output to a file instead of stdout.
	emitOutputFlag = cliflags.Spec[string]{
		Name:  "output",
		Short: "o",
		Usage: "write output to this file instead of stdout",
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
