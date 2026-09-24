package cli

import (
	emitcli "github.com/go-envx/envx/app/internal/features/emit/cli"
	envcli "github.com/go-envx/envx/app/internal/features/env/cli"
	packcli "github.com/go-envx/envx/app/internal/features/pack/cli"
	runnercli "github.com/go-envx/envx/app/internal/features/runner/cli"
	secretscli "github.com/go-envx/envx/app/internal/features/secrets/cli"
	validatecli "github.com/go-envx/envx/app/internal/features/validate/cli"
	"github.com/go-envx/envx/app/internal/features/workspace"
	workspacecli "github.com/go-envx/envx/app/internal/features/workspace/cli"
	"github.com/go-envx/envx/app/internal/utils/cliflags"
	"github.com/spf13/cobra"
)

const (
	rootUsage = "envx [command] [flags]"
	rootShort = "envx is a CLI tool for managing environment variables"
)

// NewRootCmd builds the command tree. It registers the persistent --config flag,
// which every action reads back through core.GetInput to locate the manifest.
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

	cliflags.BindString(root.PersistentFlags(), new(string), &workspace.ConfigFlag)

	root.AddCommand(
		workspacecli.NewCreateCmd(workspace.NewCreateWorkspaceHandler()),
		envcli.NewGetCmd(),
		secretscli.NewKeypairCmd(),
		packcli.NewPackCmd(),
		runnercli.NewRunCmd(),
		envcli.NewSetCmd(),
		envcli.NewExplainCmd(),
		emitcli.NewEmitCmd(),
		envcli.NewDiffCmd(),
		secretscli.NewSecretsCmd(),
		validatecli.NewValidateCmd(),
	)
	return root
}
