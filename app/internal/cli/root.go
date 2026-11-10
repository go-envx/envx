package cli

import (
	"github.com/go-envx/envx/app/internal/app"
	emitcli "github.com/go-envx/envx/app/internal/core/emit/cli"
	envcli "github.com/go-envx/envx/app/internal/core/env/cli"
	packcli "github.com/go-envx/envx/app/internal/core/pack/cli"
	scaffoldcli "github.com/go-envx/envx/app/internal/core/scaffold/cli"
	secretscli "github.com/go-envx/envx/app/internal/core/secrets/cli"
	validatecli "github.com/go-envx/envx/app/internal/core/validate/cli"
	workspacecli "github.com/go-envx/envx/app/internal/core/workspace/cli"
	"github.com/spf13/cobra"
)

const (
	rootUsage = "envx [command] [flags]"
	rootShort = "envx is a CLI tool for managing environment variables"
)

// NewRootCmd builds the command tree. It registers the persistent --config flag,
// which every action reads back by name to locate the manifest.
// The build metadata in info is rendered by the --version flag.
func NewRootCmd(info BuildInfo) (*cobra.Command, error) {
	factory := app.Factory{}

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

	workspacecli.BindConfig(root)

	root.AddCommand(
		scaffoldcli.NewCreateCommand(factory),
		envcli.NewGetCommand(factory),
		secretscli.NewKeypairCommand(factory),
		packcli.NewPackCommand(factory),
		envcli.NewRunCommand(factory),
		envcli.NewSetCommand(factory),
		envcli.NewExplainCommand(factory),
		emitcli.NewEmitCommand(factory),
		envcli.NewDiffCommand(factory),
		secretscli.NewSecretsCommand(factory),
		validatecli.NewValidateCommand(factory),
	)
	return root, nil
}
