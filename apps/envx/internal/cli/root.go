package cli

import (
	"github.com/go-envx/envx/apps/envx/internal/actions/diff"
	"github.com/go-envx/envx/apps/envx/internal/actions/explain"
	"github.com/go-envx/envx/apps/envx/internal/actions/get"
	"github.com/go-envx/envx/apps/envx/internal/actions/run"
	"github.com/go-envx/envx/apps/envx/internal/actions/set"
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/go-envx/envx/apps/envx/internal/settings"
	"github.com/spf13/cobra"
)

const (
	rootUsage = "envx [command] [flags]"
	rootShort = "envx is a CLI tool for managing environment variables"
)

// -------------------------------------------------------------------------------------

// NewRootCmd builds the command tree. It registers the persistent --config flag
// and forwards its address to every action, which loads and resolves the
// manifest on demand. version is injected from main at build time.
func NewRootCmd(version string) *cobra.Command {
	var configPath string

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

	flags.BindPersistentString(root, &configPath, &settings.Config)

	root.AddCommand(
		get.NewCommand(&configPath),
		run.NewCommand(&configPath),
		set.NewCommand(&configPath),
		explain.NewCommand(&configPath),
		diff.NewCommand(&configPath),
	)
	return root
}
