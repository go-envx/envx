package cli

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/core"
	emitcli "github.com/go-envx/envx/app/internal/features/emit/cli"
	envcli "github.com/go-envx/envx/app/internal/features/env/cli"
	packcli "github.com/go-envx/envx/app/internal/features/pack/cli"
	runnercli "github.com/go-envx/envx/app/internal/features/runner/cli"
	scaffoldcli "github.com/go-envx/envx/app/internal/features/scaffold/cli"
	secretscli "github.com/go-envx/envx/app/internal/features/secrets/cli"
	validatecli "github.com/go-envx/envx/app/internal/features/validate/cli"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/spf13/cobra"
)

const (
	rootUsage = "envx [command] [flags]"
	rootShort = "envx is a CLI tool for managing environment variables"
)

// NewRootCmd builds the command tree. It registers the persistent --config flag,
// which every action reads back through flags.Config to locate the manifest.
// The build metadata in info is rendered by the --version flag.
func NewRootCmd(info BuildInfo) (*cobra.Command, error) {
	appFactory, err := core.NewAppFactory()
	if err != nil {
		return nil, fmt.Errorf("composing application: %w", err)
	}

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

	flags.Bind(root.PersistentFlags(), &flags.Config)

	root.AddCommand(
		scaffoldcli.NewCreateCommand(appFactory),
		envcli.NewGetCommand(appFactory),
		secretscli.NewKeypairCmd(appFactory),
		packcli.NewPackCommand(appFactory),
		runnercli.NewRunCommand(appFactory),
		envcli.NewSetCommand(appFactory),
		envcli.NewExplainCommand(appFactory),
		emitcli.NewEmitCommand(appFactory),
		envcli.NewDiffCommand(appFactory),
		secretscli.NewSecretsCommand(appFactory),
		validatecli.NewValidateCommand(appFactory),
	)
	return root, nil
}
