// Package flags provides flag specifications, common presentation flags,
// and binding helpers for pflag flag sets.
package flags

// Shared presentation and global flags.
var (
	// Config selects the manifest path (auto-discovered when unset).
	Config = Spec[string]{
		Name:  "config",
		Env:   "ENVX_CONFIG",
		Usage: "path to envx.yaml, or a directory containing it",
	}

	// Output selects the rendering format for tabular commands.
	Output = Spec[string]{
		Name:  "output",
		Short: "o",
		Usage: "output format: table|json",
	}

	// Verbose prints additional detail in command output.
	Verbose = Spec[bool]{
		Name:  "verbose",
		Short: "v",
		Usage: "print detailed output",
	}
)
