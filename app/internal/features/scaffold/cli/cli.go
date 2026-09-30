// Package cli provides CLI commands for workspace scaffolding.
package cli

import (
	"github.com/go-envx/envx/app/internal/features/scaffold"
	"github.com/go-envx/envx/app/internal/shared/flags"
)

// Flags local to scaffold commands.
var (
	// forceFlag overwrites existing files instead of stopping on conflicts.
	forceFlag = flags.Spec[bool]{
		Name:  "force",
		Usage: "overwrite existing files",
	}

	// targetDirFlag is the directory to scaffold into.
	targetDirFlag = flags.Spec[string]{
		Name:  "target-dir",
		Usage: "directory to scaffold into",
	}
)

// targetDirFor returns a targetDirFlag specification defaulted to templateName.
func targetDirFor(templateName string) *flags.Spec[string] {
	spec := targetDirFlag
	spec.Default = templateName
	return &spec
}

// Factory defines the capabilities required for the scaffold commands.
type Factory interface {
	ScaffoldService() (*scaffold.Service, error)
}
