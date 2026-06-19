// Package actions holds thin cobra-aware wiring shared by the env-resolving
// actions. Its single job is to bind the engine settings flag group ONCE, at the
// action edge, so the engine package itself never imports cobra.
package actions

import (
	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/spf13/cobra"
)

// -------------------------------------------------------------------------------------
// RegisterEngineFlags binds the engine settings group (strict/prefix/suffix/
// namespace-prefix) onto cmd, writing parsed values into dst. Names and usage
// come from the flags catalog; registration goes through cmd.Flags(), so this
// file imports cobra and flags but never pflag directly.
func RegisterEngineFlags(cmd *cobra.Command, dst *engine.Flags) {
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
