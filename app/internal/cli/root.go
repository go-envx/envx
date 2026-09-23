package cli

import (
	"github.com/go-envx/envx/app/internal/actions/diff"
	"github.com/go-envx/envx/app/internal/actions/explain"
	"github.com/go-envx/envx/app/internal/actions/get"
	"github.com/go-envx/envx/app/internal/actions/set"
	"github.com/go-envx/envx/app/internal/actions/validate"
	emitcli "github.com/go-envx/envx/app/internal/features/emit/cli"
	packcli "github.com/go-envx/envx/app/internal/features/pack/cli"
	runnercli "github.com/go-envx/envx/app/internal/features/runner/cli"
	secretscli "github.com/go-envx/envx/app/internal/features/secrets/cli"
	"github.com/go-envx/envx/app/internal/features/workspace"
	workspacecli "github.com/go-envx/envx/app/internal/features/workspace/cli"
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/spf13/cobra"
)

const (
	rootUsage = "envx [command] [flags]"
	rootShort = "envx is a CLI tool for managing environment variables"
)

// NewRootCmd builds the command tree. It registers the persistent --config flag,
// which every action reads back through flags.GetInput to locate the manifest.
// The build metadata in info is rendered by the --version flag.
func NewRootCmd(info BuildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:           rootUsage,
		Short:         rootShort,
		Version:       formatVersion(info),
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Parsing and validation have already succeeded by the time this
			// runs, so silence Cobra's usage dump on any later (runtime) error;
			// usage and validation errors happen earlier and still show help.
			// main.go reads this same flag to map the process exit code.
			cmd.SilenceUsage = true
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	flags.Register(root.PersistentFlags(),
		flags.WithConfig,
	)

	root.AddCommand(
		workspacecli.NewCreateCmd(workspace.NewCreateWorkspaceHandler()),
		get.NewCommand(),
		secretscli.NewKeypairCmd(),
		packcli.NewPackCmd(),
		runnercli.NewRunCmd(),
		set.NewCommand(),
		explain.NewCommand(),
		emitcli.NewEmitCmd(),
		diff.NewCommand(),
		secretscli.NewSecretsCmd(),
		validate.NewCommand(),
	)
	return root
}
