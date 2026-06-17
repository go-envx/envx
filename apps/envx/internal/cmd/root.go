package cmd

import (
	"github.com/go-envx/envx/apps/envx/internal/app"
	"github.com/spf13/cobra"
)

const (
	rootUsage = "envx [command] [flags]"
	rootShort = "envx is a CLI tool for managing environment variables."
	rootFlag  = "path to envx.yaml manifest (env: ENVX_CONFIG, default: auto-discover)"
	envFlag   = "target environment (env: ENVX_ENV, default: development)"
)

// -------------------------------------------------------------------------------------
// NewRootCmd builds the top-level cobra command tree for envx. It registers
// persistent flags (like --config) that are inherited by all subcommands, and
// wires up each subcommand. Version is injected from main.go at build time.
func NewRootCmd(version string) *cobra.Command {
	var configPath string
	var envName string
	application := app.New()

	cmd := &cobra.Command{
		Use:           rootUsage,
		Short:         rootShort,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		// When invoked without a subcommand, print help text so the user
		// can discover available commands and flags.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// --config: persistent flag inherited by all subcommands. Allows the
	// user to explicitly specify the manifest path instead of relying on
	// auto-discovery. Also settable via ENVX_CONFIG env var.
	cmd.PersistentFlags().StringVar(&configPath, "config", "", rootFlag)

	// --env: persistent flag inherited by all subcommands. Sets the target
	// environment. Also settable via ENVX_ENV env var, manifest
	// default_environment, or defaults to "development".
	cmd.PersistentFlags().StringVar(&envName, "env", "", envFlag)

	// Register subcommands. Each receives the shared App instance and
	// pointers to persistent flag values so they can be resolved after
	// flag parsing.
	cmd.AddCommand(newRunCmd(application, &configPath, &envName))
	cmd.AddCommand(newGetCmd(application, &configPath, &envName))
	cmd.AddCommand(newSetCmd(application, &configPath, &envName))

	return cmd
}
