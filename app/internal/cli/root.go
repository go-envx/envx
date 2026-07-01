package cli

import (
	"github.com/go-envx/envx/app/internal/actions/diff"
	"github.com/go-envx/envx/app/internal/actions/explain"
	"github.com/go-envx/envx/app/internal/actions/get"
	"github.com/go-envx/envx/app/internal/actions/run"
	"github.com/go-envx/envx/app/internal/actions/set"
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/spf13/cobra"
)

const (
	rootUsage = "envx [command] [flags]"
	rootShort = "envx is a CLI tool for managing environment variables"
)

// -------------------------------------------------------------------------------------

// NewRootCmd builds the command tree. It registers the persistent --config flag,
// which every action reads back through flags.GetInput to locate the manifest.
// version is injected from main at build time.
func NewRootCmd(version string) *cobra.Command {
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

	flags.Register(root.PersistentFlags(),
		flags.WithConfig,
	)

	root.AddCommand(
		get.NewCommand(),
		run.NewCommand(),
		set.NewCommand(),
		explain.NewCommand(),
		diff.NewCommand(),
	)
	return root
}
