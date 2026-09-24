package cli

import "github.com/go-envx/envx/app/internal/utils/cliflags"

// Flags local to pack command.
var (
	// Out selects the destination directory a bundle is written to.
	Out = cliflags.FlagSpec{
		Name:  "out",
		Usage: "destination directory for the packed workspace",
	}

	// Env selects one or more environments to include in a pack bundle.
	Env = cliflags.FlagSpec{
		Name:  "env",
		Short: "e",
		Usage: "environment to include (repeatable; default: all declared)",
	}

	// Project narrows a pack bundle to one or more projects' includes.
	Project = cliflags.FlagSpec{
		Name:  "project",
		Short: "p",
		Usage: "limit the bundle to this project's includes (repeatable; default: all)",
	}

	// Force replaces an existing non-empty output directory instead of refusing it.
	Force = cliflags.FlagSpec{
		Name:  "force",
		Usage: "replace the output directory if it already exists and is not empty",
	}
)
