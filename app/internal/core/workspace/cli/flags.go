package cli

import (
	"slices"

	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/cliflags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// configFlag selects the manifest path (auto-discovered when unset).
	configFlag = cliflags.Spec[string]{
		Name:  "config",
		Env:   "ENVX_CONFIG",
		Usage: "path to envx.yaml, or a directory containing it",
	}

	// envFlag selects the target environment.
	envFlag = cliflags.Spec[string]{
		Name:  "env",
		Short: "E",
		Env:   "ENVX_ENV",
		Usage: "target environment (defaults to first declared environment in envx.yaml)",
	}

	// requireOverlaysFlag requires every environment overlay file to exist.
	requireOverlaysFlag = cliflags.Spec[bool]{
		Name:  "require-overlays",
		Env:   "ENVX_REQUIRE_OVERLAYS",
		Usage: "require all environment overlay files to exist",
	}

	// prefixFlag is prepended to every resolved key.
	prefixFlag = cliflags.Spec[string]{
		Name:  "prefix",
		Env:   "ENVX_PREFIX",
		Usage: "prefix prepended to every key",
	}

	// suffixFlag is appended to every resolved key.
	suffixFlag = cliflags.Spec[string]{
		Name:  "suffix",
		Env:   "ENVX_SUFFIX",
		Usage: "suffix appended to every key",
	}

	// delimiterFlag joins a list-valued leaf into a single value.
	delimiterFlag = cliflags.Spec[string]{
		Name:  "delimiter",
		Env:   "ENVX_DELIMITER",
		Usage: `string used to join list values (default ",")`,
	}

	// overloadFlag lets file values override existing host variables.
	overloadFlag = cliflags.Spec[bool]{
		Name:  "overload",
		Env:   "ENVX_OVERLOAD",
		Usage: "file values override OS env vars",
	}

	// referencePatternFlag overrides the {{VAR}} reference syntax with a regex.
	referencePatternFlag = cliflags.Spec[string]{
		Name:  "reference-pattern",
		Env:   "ENVX_REFERENCE_PATTERN",
		Usage: "regex overriding the {{VAR}} reference syntax (group 1 is the name)",
	}
)

// BindConfig registers the persistent --config flag on cmd.
func BindConfig(cmd *cobra.Command) {
	configFlag.BindPersistent(cmd)
}

// ConfigPath returns the manifest path specified by the --config flag or
// ENVX_CONFIG environment variable, or empty when unset.
func ConfigPath(cmd *cobra.Command) string {
	return configFlag.Value(cmd)
}

// BindEnv registers only the --env setting flag on cmd.
func BindEnv(cmd *cobra.Command) {
	envFlag.Bind(cmd)
}

// BindSettings registers the workspace setting flags on cmd, except those
// whose names are listed in omit.
func BindSettings(cmd *cobra.Command, omit ...string) {
	all := &cobra.Command{}
	delimiterFlag.Bind(all)
	envFlag.Bind(all)
	overloadFlag.Bind(all)
	prefixFlag.Bind(all)
	referencePatternFlag.Bind(all)
	requireOverlaysFlag.Bind(all)
	suffixFlag.Bind(all)

	all.Flags().VisitAll(func(f *pflag.Flag) {
		if !slices.Contains(omit, f.Name) {
			cmd.Flags().AddFlag(f)
		}
	})
}

// Settings returns the settings given by flag or ENVX_* variable on cmd.
// Unset or unregistered settings stay nil so the manifest layers apply.
func Settings(cmd *cobra.Command) workspace.Settings {
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
