package cmd

import (
	"github.com/spf13/cobra"
)

// NewRootCmd constructs the root command tree. Version is injected from main.
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "envx",
		Short:         "envx is a CLI tool for managing environment variables.",
		Long:          "envx is a CLI tool for managing environment variables in both monorepos and single repositories.",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Register subcommands here as they are added.

	return cmd
}
