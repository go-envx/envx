// Package cli provides the CLI command that renders a resolved environment to
// a delivery target.
package cli

import (
	"github.com/go-envx/envx/app/internal/core/emit"
	"github.com/go-envx/envx/app/internal/lib/cliflags"
)

// Flags local to the emit command.
var (
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

	// outputFlag writes the rendered output to a file instead of stdout.
	outputFlag = cliflags.Spec[string]{
		Name:  "output",
		Short: "o",
		Usage: "write output to this file instead of stdout",
	}
)

// Factory defines the capabilities required for the emit command.
type Factory interface {
	NewEmitService(configPath string) (*emit.Service, error)
}
