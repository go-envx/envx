// Package cli provides CLI commands for workspace scaffolding.
package cli

import (
	"github.com/go-envx/envx/app/internal/core/scaffold"
	"github.com/go-envx/envx/app/internal/lib/cliflags"
)

// Flags local to scaffold commands.
var (
	// forceFlag overwrites existing files instead of stopping on conflicts.
	forceFlag = cliflags.Spec[bool]{
		Name:  "force",
		Usage: "overwrite existing files",
	}

	// targetDirFlag is the directory to scaffold into.
	targetDirFlag = cliflags.Spec[string]{
		Name:  "target-dir",
		Usage: "directory to scaffold into",
	}
)

// templateSpec describes the metadata for a single template subcommand.
type templateSpec struct {
	Name  string
	Short string
	Long  string
}

func (t templateSpec) targetDirFlag() cliflags.Spec[string] {
	spec := targetDirFlag
	spec.Default = t.Name
	return spec
}

// Factory defines the capabilities required for the scaffold commands.
type Factory interface {
	NewScaffoldService() (*scaffold.Service, error)
}
