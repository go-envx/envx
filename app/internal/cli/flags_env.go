package cli

import (
	"slices"

	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/flagspec"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Environment setting flags shared by commands that resolve an environment.
var (
	// envFlag selects the target environment.
	envFlag = flagspec.Spec[string]{
		Name:  "env",
		Short: "E",
		Env:   "ENVX_ENV",
		Usage: "target environment (defaults to first declared environment in envx.yaml)",
	}

	// requireOverlaysFlag requires every environment overlay file to exist.
	requireOverlaysFlag = flagspec.Spec[bool]{
		Name:  "require-overlays",
		Env:   "ENVX_REQUIRE_OVERLAYS",
		Usage: "require all environment overlay files to exist",
	}

	// prefixFlag is prepended to every resolved key.
	prefixFlag = flagspec.Spec[string]{
		Name:  "prefix",
		Env:   "ENVX_PREFIX",
		Usage: "prefix prepended to every key",
	}

	// suffixFlag is appended to every resolved key.
	suffixFlag = flagspec.Spec[string]{
		Name:  "suffix",
		Env:   "ENVX_SUFFIX",
		Usage: "suffix appended to every key",
	}

	// delimiterFlag joins a list-valued leaf into a single value.
	delimiterFlag = flagspec.Spec[string]{
		Name:  "delimiter",
		Env:   "ENVX_DELIMITER",
		Usage: `string used to join list values (default ",")`,
	}

	// overloadFlag lets file values override existing host variables.
	overloadFlag = flagspec.Spec[bool]{
		Name:  "overload",
		Env:   "ENVX_OVERLOAD",
		Usage: "file values override OS env vars",
	}

	// referencePatternFlag overrides the {{VAR}} reference syntax with a regex.
	referencePatternFlag = flagspec.Spec[string]{
		Name:  "reference-pattern",
		Env:   "ENVX_REFERENCE_PATTERN",
		Usage: "regex overriding the {{VAR}} reference syntax (group 1 is the name)",
	}
)

// bindEnvFlags registers the environment setting flags on fs, except those
// whose names are listed in omit.
func bindEnvFlags(fs *pflag.FlagSet, omit ...string) {
	all := pflag.NewFlagSet("env", pflag.ContinueOnError)
	delimiterFlag.Bind(all)
	envFlag.Bind(all)
	overloadFlag.Bind(all)
	prefixFlag.Bind(all)
	referencePatternFlag.Bind(all)
	requireOverlaysFlag.Bind(all)
	suffixFlag.Bind(all)

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
		Delimiter:        delimiterFlag.Lookup(cmd),
		Env:              envFlag.Lookup(cmd),
		Overload:         overloadFlag.Lookup(cmd),
		Prefix:           prefixFlag.Lookup(cmd),
		ReferencePattern: referencePatternFlag.Lookup(cmd),
		RequireOverlays:  requireOverlaysFlag.Lookup(cmd),
		Suffix:           suffixFlag.Lookup(cmd),
	}
}
