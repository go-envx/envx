package cli

import "github.com/go-envx/envx/app/internal/lib/flagspec"

// Flags shared across commands.
var (
	// outputFlag selects the rendering format for tabular commands.
	//nolint:unused // Relocated to core/env/cli in Phase 6.3.
	outputFlag = flagspec.Spec[string]{
		Name:  "output",
		Short: "o",
		Usage: "output format: table|json",
	}

	// verboseFlag prints additional detail in command output.
	//nolint:unused // Relocated to command packages in Phase 6.3.
	verboseFlag = flagspec.Spec[bool]{
		Name:  "verbose",
		Short: "v",
		Usage: "print detailed output",
	}

	// revealFlag decrypts referenced secret values instead of masking them.
	//nolint:unused // Relocated to command packages in Phase 6.3.
	revealFlag = flagspec.Spec[bool]{
		Name:  "reveal",
		Usage: "decrypt secret references instead of masking them",
	}
)
