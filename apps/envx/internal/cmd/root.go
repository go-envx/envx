package cmd

import (
	"context"

	"github.com/spf13/cobra"
)

// version can be injected at build time using ldflags
var version = "0.1.0-dev"

var rootCmd = &cobra.Command{
	Use:           "envx",
	Short:         "envx is a CLI tool for managing environments",
	Long:          `envx is a highly maintainable Cobra CLI built following Go best practices.`,
	Version:       version,
	SilenceErrors: true, // Errors are handled centrally in main.go
	SilenceUsage:  true, // Do not print usage automatically on command errors
	RunE: func(cmd *cobra.Command, args []string) error {
		// When called without subcommands, just print help
		return cmd.Help()
	},
}

// ExecuteContext brings the Cobra CLI to life
func ExecuteContext(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}
