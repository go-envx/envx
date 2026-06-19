// Package cli builds the root "envx" cobra command and registers each action.
// It owns the persistent --config and --env flags and resolves the shared
// config.Global exactly once in PersistentPreRunE, so every action observes the
// same immutable root context.
package cli

import (
	"github.com/go-envx/envx/apps/envx/internal/actions/diff"
	"github.com/go-envx/envx/apps/envx/internal/actions/explain"
	"github.com/go-envx/envx/apps/envx/internal/actions/get"
	"github.com/go-envx/envx/apps/envx/internal/actions/run"
	"github.com/go-envx/envx/apps/envx/internal/actions/set"
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/spf13/cobra"
)

const (
	rootUsage = "envx [command] [flags]"
	rootShort = "envx is a CLI tool for managing environment variables"
)

// -------------------------------------------------------------------------------------
// NewRootCmd builds the command tree. It registers the persistent --config and
// --env flags, resolves the shared config.Global once in PersistentPreRunE
// (skipped for help/version and for the bare root), and hands a *config.Global
// to every action so they share one immutable root context. version is injected
// from main at build time.
func NewRootCmd(version string) *cobra.Command {
	var (
		configPath string
		envName    string
		global     config.Global
	)

	root := &cobra.Command{
		Use:           rootUsage,
		Short:         rootShort,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	pf := root.PersistentFlags()
	pf.StringVarP(
		&configPath, flags.Config.Name, flags.Config.Short, "",
		flags.Config.HelpText(),
	)
	pf.StringVarP(
		&envName, flags.Env.Name, flags.Env.Short, "", flags.Env.HelpText(),
	)

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		// The bare root (help/usage) needs no manifest; only subcommands do.
		if !cmd.HasParent() {
			return nil
		}
		g, err := config.Resolve(configPath, envName, cmd.Flags())
		if err != nil {
			return err
		}
		global = g
		return nil
	}

	root.AddCommand(
		get.NewCommand(&global),
		run.NewCommand(&global),
		set.NewCommand(&global),
		explain.NewCommand(&global),
		diff.NewCommand(&global),
	)
	return root
}
