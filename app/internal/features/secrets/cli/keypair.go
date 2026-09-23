package cli

import (
	"github.com/go-envx/envx/app/internal/features/secrets/cli/generate"
	"github.com/go-envx/envx/app/internal/features/secrets/cli/inspect"
	"github.com/go-envx/envx/app/internal/features/secrets/cli/print"
	"github.com/go-envx/envx/app/internal/features/secrets/cli/rotate"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	keypairUsage = "keypair [command]"
	keypairShort = "Manage a secret group's keypair"
	keypairLong  = `
		Generate, inspect, or rotate the asymmetric keypair for one secret group, or
		print an unassigned pair without storing it. Managed public keys are stored in
		secrets.yaml; private keys are stored in the configured git-ignored envx.keys
		file.
	`
)

// NewKeypairCmd builds the "keypair" parent command and registers its subcommands.
func NewKeypairCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   keypairUsage,
		Short: keypairShort,
		Long:  str.Dedent(keypairLong),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		generate.NewCommand(),
		inspect.NewCommand(),
		print.NewCommand(),
		rotate.NewCommand(),
	)
	return cmd
}
