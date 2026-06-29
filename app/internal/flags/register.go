package flags

import (
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/spf13/cobra"
)

// -------------------------------------------------------------------------------------

// BindString registers spec as a string flag on cmd's local flag set, sourcing
// the name, shorthand, and usage from the spec so registration can never
// disagree with resolution, and writes the parsed value into dst.
func BindString(cmd *cobra.Command, dst *string, spec *schema.FlagSpec) {
	cmd.Flags().StringVarP(dst, spec.Name, spec.Short, "", spec.HelpText())
}

// -------------------------------------------------------------------------------------

// BindBool registers spec as a bool flag on cmd's local flag set, sourcing the
// name, shorthand, and usage from the spec so registration can never
// disagree with resolution, and writes the parsed value into dst.
func BindBool(cmd *cobra.Command, dst *bool, spec *schema.FlagSpec) {
	cmd.Flags().BoolVarP(dst, spec.Name, spec.Short, false, spec.HelpText())
}

// -------------------------------------------------------------------------------------

// BindPersistentString registers spec as a persistent string flag on cmd (so it
// applies to every subcommand), sourcing the name, shorthand, and usage
// from the spec, and writes the parsed value into dst. It is the persistent
// counterpart to BindString, used for root-level flags like --config.
func BindPersistentString(cmd *cobra.Command, dst *string, spec *schema.FlagSpec) {
	cmd.PersistentFlags().StringVarP(dst, spec.Name, spec.Short, "", spec.HelpText())
}

// -------------------------------------------------------------------------------------

// OptionalString returns a pointer to val when the user explicitly set spec's
// flag on cmd, and nil otherwise. It translates cobra's changed state into the
// optional form config.Input expects, keeping cobra knowledge at the CLI edge.
func OptionalString(cmd *cobra.Command, spec *schema.FlagSpec, val string) *string {
	if !cmd.Flags().Changed(spec.Name) {
		return nil
	}
	return &val
}

// -------------------------------------------------------------------------------------

// OptionalBool returns a pointer to val when the user explicitly set spec's flag
// on cmd, and nil otherwise. It is the boolean counterpart to OptionalString.
func OptionalBool(cmd *cobra.Command, spec *schema.FlagSpec, val bool) *bool {
	if !cmd.Flags().Changed(spec.Name) {
		return nil
	}
	return &val
}
