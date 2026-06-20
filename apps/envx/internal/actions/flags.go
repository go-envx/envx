package actions

import (
	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/spf13/cobra"
)

// -------------------------------------------------------------------------------------
// RegisterEngineFlags binds the engine settings group (strict/prefix/suffix/
// namespace-prefix) onto cmd, writing parsed values into dst. The --env flag is
// registered separately (see RegisterEnvFlag) so positional-environment commands
// like diff stay free of it. Names and usage come from the flags catalog;
// registration goes through cmd.Flags(), so this file imports cobra and flags but
// never pflag directly.
func RegisterEngineFlags(cmd *cobra.Command, dst *engine.Settings) {
	fs := cmd.Flags()
	fs.BoolVarP(
		&dst.Strict, flags.Strict.Name, flags.Strict.Short, false,
		flags.Strict.HelpText(),
	)
	fs.StringVarP(
		&dst.Prefix, flags.Prefix.Name, flags.Prefix.Short, "",
		flags.Prefix.HelpText(),
	)
	fs.StringVarP(
		&dst.Suffix, flags.Suffix.Name, flags.Suffix.Short, "",
		flags.Suffix.HelpText(),
	)
	fs.BoolVarP(
		&dst.NamespacePrefix, flags.NamespacePrefix.Name,
		flags.NamespacePrefix.Short, false, flags.NamespacePrefix.HelpText(),
	)
}

// -------------------------------------------------------------------------------------
// RegisterEnvFlag binds the --env flag onto cmd, writing the raw value into dst.
// It is registered per-action (get/run/explain/set) rather than as part of the
// engine settings group, so positional-environment commands like diff never gain
// an --env flag.
func RegisterEnvFlag(cmd *cobra.Command, dst *string) {
	cmd.Flags().StringVarP(
		dst, flags.Env.Name, flags.Env.Short, "", flags.Env.HelpText(),
	)
}
