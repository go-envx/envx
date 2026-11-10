package cli

import (
	"slices"

	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Environment setting flags shared by commands that resolve an environment.
var (
	// envFlag selects the target environment.
	envFlag = flagSpec[string]{
		name:   "env",
		short:  "E",
		envVar: "ENVX_ENV",
		usage:  "target environment (defaults to first declared environment in envx.yaml)",
	}

	// requireOverlaysFlag requires every environment overlay file to exist.
	requireOverlaysFlag = flagSpec[bool]{
		name:   "require-overlays",
		envVar: "ENVX_REQUIRE_OVERLAYS",
		usage:  "require all environment overlay files to exist",
	}

	// prefixFlag is prepended to every resolved key.
	prefixFlag = flagSpec[string]{
		name:   "prefix",
		envVar: "ENVX_PREFIX",
		usage:  "prefix prepended to every key",
	}

	// suffixFlag is appended to every resolved key.
	suffixFlag = flagSpec[string]{
		name:   "suffix",
		envVar: "ENVX_SUFFIX",
		usage:  "suffix appended to every key",
	}

	// delimiterFlag joins a list-valued leaf into a single value.
	delimiterFlag = flagSpec[string]{
		name:   "delimiter",
		envVar: "ENVX_DELIMITER",
		usage:  `string used to join list values (default ",")`,
	}

	// overloadFlag lets file values override existing host variables.
	overloadFlag = flagSpec[bool]{
		name:   "overload",
		envVar: "ENVX_OVERLOAD",
		usage:  "file values override OS env vars",
	}

	// referencePatternFlag overrides the {{VAR}} reference syntax with a regex.
	referencePatternFlag = flagSpec[string]{
		name:   "reference-pattern",
		envVar: "ENVX_REFERENCE_PATTERN",
		usage:  "regex overriding the {{VAR}} reference syntax (group 1 is the name)",
	}
)

// bindEnvFlags registers the environment setting flags on fs, except those
// whose names are listed in omit.
func bindEnvFlags(fs *pflag.FlagSet, omit ...string) {
	all := pflag.NewFlagSet("env", pflag.ContinueOnError)
	delimiterFlag.bind(all)
	envFlag.bind(all)
	overloadFlag.bind(all)
	prefixFlag.bind(all)
	referencePatternFlag.bind(all)
	requireOverlaysFlag.bind(all)
	suffixFlag.bind(all)

	all.VisitAll(func(f *pflag.Flag) {
		if !slices.Contains(omit, f.Name) {
			fs.AddFlag(f)
		}
	})
}

// envOverrides returns the settings given by flag or ENVX_* variable on cmd.
// Unset or unregistered settings stay nil so the manifest layers apply.
func envOverrides(cmd *cobra.Command) workspace.Settings {
	return workspace.Settings{
		Delimiter:        delimiterFlag.lookup(cmd),
		Env:              envFlag.lookup(cmd),
		Overload:         overloadFlag.lookup(cmd),
		Prefix:           prefixFlag.lookup(cmd),
		ReferencePattern: referencePatternFlag.lookup(cmd),
		RequireOverlays:  requireOverlaysFlag.lookup(cmd),
		Suffix:           suffixFlag.lookup(cmd),
	}
}
