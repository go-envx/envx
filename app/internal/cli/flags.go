package cli

// Flags shared across commands.
var (
	// configFlag selects the manifest path (auto-discovered when unset).
	configFlag = flagSpec[string]{
		name:   "config",
		envVar: "ENVX_CONFIG",
		usage:  "path to envx.yaml, or a directory containing it",
	}

	// outputFlag selects the rendering format for tabular commands.
	outputFlag = flagSpec[string]{
		name:  "output",
		short: "o",
		usage: "output format: table|json",
	}

	// verboseFlag prints additional detail in command output.
	verboseFlag = flagSpec[bool]{
		name:  "verbose",
		short: "v",
		usage: "print detailed output",
	}

	// revealFlag decrypts referenced secret values instead of masking them.
	revealFlag = flagSpec[bool]{
		name:  "reveal",
		usage: "decrypt secret references instead of masking them",
	}
)
