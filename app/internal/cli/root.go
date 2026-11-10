package cli

import (
	emitcli "github.com/go-envx/envx/app/internal/core/emit/cli"
	envcli "github.com/go-envx/envx/app/internal/core/env/cli"
	packcli "github.com/go-envx/envx/app/internal/core/pack/cli"
	scaffoldcli "github.com/go-envx/envx/app/internal/core/scaffold/cli"
	secretscli "github.com/go-envx/envx/app/internal/core/secrets/cli"
	validatecli "github.com/go-envx/envx/app/internal/core/validate/cli"
	workspacecli "github.com/go-envx/envx/app/internal/core/workspace/cli"
	"github.com/go-envx/envx/app/internal/lib/stdio"
	"github.com/spf13/cobra"
)

const (
	rootUsage = "envx [command] [flags]"
	rootShort = "envx is a CLI tool for managing environment variables"
)

// appFactory aggregates every command package's service factory.
type appFactory interface {
	emitcli.Factory
	envcli.Factory
	packcli.Factory
	scaffoldcli.Factory
	secretscli.Factory
	validatecli.Factory
}

// NewRootCommand builds the command tree. It registers the persistent --config
// flag, renders info for --version, and installs the normalized streams so
// handlers can read them through Cobra's effective stream accessors.
func NewRootCommand(
	info BuildInfo,
	factory appFactory,
	streams stdio.Streams,
) *cobra.Command {
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

	streams = streams.Normalized()
	root.SetIn(streams.In)
	root.SetOut(streams.Out)
	root.SetErr(streams.ErrOut)

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
	return root
}
