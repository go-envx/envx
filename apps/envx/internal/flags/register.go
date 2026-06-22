package flags

import (
	"github.com/spf13/cobra"
)

// -------------------------------------------------------------------------------------
// NewConfigFlag binds the --config flag onto cmd as a persistent flag (so it
// applies to every subcommand), writing the parsed path into dst.
func NewConfigFlag(cmd *cobra.Command, dst *string) {
	def, _ := Config.Default.(string)
	cmd.PersistentFlags().StringVarP(
		dst, Config.Name, Config.Short, def, Config.HelpText(),
	)
}

// -------------------------------------------------------------------------------------
// NewEnvFlag binds the --env flag onto cmd, writing the parsed value into dst.
// Actions register it individually (get/run/explain/set), so positional-
// environment commands like diff can simply omit it.
func NewEnvFlag(cmd *cobra.Command, dst *string) {
	bindString(cmd, dst, &Env)
}

// -------------------------------------------------------------------------------------
// NewStrictFlag binds the --strict flag onto cmd, writing the parsed value into
// dst.
func NewStrictFlag(cmd *cobra.Command, dst *bool) {
	bindBool(cmd, dst, &Strict)
}

// -------------------------------------------------------------------------------------
// NewPrefixFlag binds the --prefix flag onto cmd, writing the parsed value into
// dst.
func NewPrefixFlag(cmd *cobra.Command, dst *string) {
	bindString(cmd, dst, &Prefix)
}

// -------------------------------------------------------------------------------------
// NewSuffixFlag binds the --suffix flag onto cmd, writing the parsed value into
// dst.
func NewSuffixFlag(cmd *cobra.Command, dst *string) {
	bindString(cmd, dst, &Suffix)
}

// -------------------------------------------------------------------------------------
// NewNamespacePrefixFlag binds the --namespace-prefix flag onto cmd, writing the
// parsed value into dst.
func NewNamespacePrefixFlag(cmd *cobra.Command, dst *bool) {
	bindBool(cmd, dst, &NamespacePrefix)
}

// -------------------------------------------------------------------------------------
// NewOverloadFlag binds the --overload flag onto cmd, writing the parsed value
// into dst.
func NewOverloadFlag(cmd *cobra.Command, dst *bool) {
	bindBool(cmd, dst, &Overload)
}

// -------------------------------------------------------------------------------------
// NewRevealFlag binds the --reveal flag onto cmd, writing the parsed value into
// dst.
func NewRevealFlag(cmd *cobra.Command, dst *bool) {
	bindBool(cmd, dst, &Reveal)
}

// -------------------------------------------------------------------------------------
// NewOutputFlag binds the --output flag onto cmd, writing the parsed value into
// dst. Its default ("table") comes straight from the Output spec.
func NewOutputFlag(cmd *cobra.Command, dst *string) {
	bindString(cmd, dst, &Output)
}

// -------------------------------------------------------------------------------------
// bindString registers spec as a string flag on cmd's local flag set, sourcing
// the name, shorthand, default, and usage from the spec so registration can
// never disagree with resolution.
func bindString(cmd *cobra.Command, dst *string, spec *Spec) {
	def, _ := spec.Default.(string)
	cmd.Flags().StringVarP(dst, spec.Name, spec.Short, def, spec.HelpText())
}

// -------------------------------------------------------------------------------------
// bindBool registers spec as a bool flag on cmd's local flag set, sourcing the
// name, shorthand, default, and usage from the spec so registration can never
// disagree with resolution.
func bindBool(cmd *cobra.Command, dst *bool, spec *Spec) {
	def, _ := spec.Default.(bool)
	cmd.Flags().BoolVarP(dst, spec.Name, spec.Short, def, spec.HelpText())
}
