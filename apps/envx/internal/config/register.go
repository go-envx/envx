package config

import (
	"github.com/go-envx/envx/apps/envx/internal/settings"
	"github.com/spf13/cobra"
)

// -------------------------------------------------------------------------------------
// BindString registers spec as a string flag on cmd's local flag set, sourcing
// the name, shorthand, default, and usage from the spec so registration can never
// disagree with resolution, and writes the parsed value into dst.
func BindString(cmd *cobra.Command, dst *string, spec *settings.Spec) {
	def, _ := spec.Default.(string)
	cmd.Flags().StringVarP(dst, spec.Name, spec.Short, def, spec.HelpText())
}

// -------------------------------------------------------------------------------------
// BindBool registers spec as a bool flag on cmd's local flag set, sourcing the
// name, shorthand, default, and usage from the spec so registration can never
// disagree with resolution, and writes the parsed value into dst.
func BindBool(cmd *cobra.Command, dst *bool, spec *settings.Spec) {
	def, _ := spec.Default.(bool)
	cmd.Flags().BoolVarP(dst, spec.Name, spec.Short, def, spec.HelpText())
}

// -------------------------------------------------------------------------------------
// BindPersistentString registers spec as a persistent string flag on cmd (so it
// applies to every subcommand), sourcing the name, shorthand, default, and usage
// from the spec, and writes the parsed value into dst. It is the persistent
// counterpart to BindString, used for root-level flags like --config.
func BindPersistentString(cmd *cobra.Command, dst *string, spec *settings.Spec) {
	def, _ := spec.Default.(string)
	cmd.PersistentFlags().StringVarP(dst, spec.Name, spec.Short, def, spec.HelpText())
}
