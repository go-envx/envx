package cmd

import (
	"github.com/go-envx/envx/apps/envx/internal/app"
	"github.com/spf13/cobra"
)

const (
	rootUsage = "envx [command] [flags]"
	rootShort = "envx is a CLI tool for managing environment variables."
	rootFlag  = "path to envx.yaml manifest (env: ENVX_CONFIG, default: auto-discover)"
)

// -------------------------------------------------------------------------------------
// NewRootCmd builds the top-level cobra command tree for envx. It registers
// persistent flags (like --config) that are inherited by all subcommands, and
// wires up each subcommand. Version is injected from main.go at build time.
func NewRootCmd(version string) *cobra.Command {
	var configPath string
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

	// Register subcommands. Each receives the shared App instance and a
	// pointer to configPath so it can be resolved after flag parsing.
	cmd.AddCommand(newRunCmd(application, &configPath))

	return cmd
}
