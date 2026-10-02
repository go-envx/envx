// Package cli implements "envx pack", which copies an environment-scoped subset
// of the workspace into an output directory that runs through the ordinary
// `envx run --config` pipeline. It selects the environments and projects to
// bundle, delegates file selection and copying to internal/features/pack, and
// renders a summary of what it wrote.
package cli

import (
	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/shared/flags"
)

// Flags local to the pack command.
var (
	// outFlag selects the destination directory a bundle is written to.
	outFlag = flags.Spec[string]{
		Name:  "out",
		Usage: "destination directory for the packed workspace",
	}

	// envFlag selects one or more environments to include in a pack bundle.
	envFlag = flags.Spec[[]string]{
		Name:  "env",
		Short: "e",
		Usage: "environment to include (repeatable; default: all declared)",
	}

	// projectFlag narrows a pack bundle to one or more projects' includes.
	projectFlag = flags.Spec[[]string]{
		Name:  "project",
		Short: "p",
		Usage: "limit the bundle to this project's includes (repeatable; default: all)",
	}

	// forceFlag replaces an existing non-empty output directory instead of
	// refusing it.
	forceFlag = flags.Spec[bool]{
		Name:  "force",
		Usage: "replace the output directory if it already exists and is not empty",
	}
)

// Factory defines the capabilities required for the pack command.
type Factory interface {
	PackService(configPath string) (*pack.Service, error)
}
