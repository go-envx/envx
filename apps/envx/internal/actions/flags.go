// Package actions holds the cobra-aware wiring shared by the env-resolving
// actions: it binds the engine settings flag group at the action edge and
// resolves the flag/env/manifest precedence into the engine.Settings the engine
// consumes, so the engine package never imports cobra and never computes
// precedence itself.
package actions

import (
	"github.com/go-envx/envx/apps/envx/internal/config"
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

// -------------------------------------------------------------------------------------
// ResolveSettings applies the settings precedence (flag > ENVX_* > project
// setting > global setting > zero, with env falling back to config.DefaultEnv)
// over the raw flag values bound at the cobra edge, producing the fully-resolved
// engine.Settings the engine consumes. The project layer is skipped when project
// is unknown; the engine surfaces the canonical "project not found" error.
func ResolveSettings(
	g *config.Global, project string, raw engine.Settings, changed config.FlagSet,
) engine.Settings {
	cfg := g.Config
	var proj config.Project
	if m, ok := cfg.LookupProject(project); ok {
		proj = m.Project
	}

	r := config.NewResolver()
	s := engine.Settings{
		Env: r.String(
			flags.Env, changed, raw.Env,
			proj.Settings.Env, cfg.Settings.Env,
		),
		Strict: r.Bool(
			flags.Strict, changed, raw.Strict,
			proj.Settings.Strict, cfg.Settings.Strict,
		),
		Prefix: r.String(
			flags.Prefix, changed, raw.Prefix,
			proj.Settings.Prefix, cfg.Settings.Prefix,
		),
		Suffix: r.String(
			flags.Suffix, changed, raw.Suffix,
			proj.Settings.Suffix, cfg.Settings.Suffix,
		),
		NamespacePrefix: r.Bool(
			flags.NamespacePrefix, changed, raw.NamespacePrefix,
			proj.Settings.NamespacePrefix, cfg.Settings.NamespacePrefix,
		),
	}
	if s.Env == "" {
		s.Env = config.DefaultEnv
	}
	return s
}
