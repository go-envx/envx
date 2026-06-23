package flags

import (
	"github.com/go-envx/envx/apps/envx/internal/schema"
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
