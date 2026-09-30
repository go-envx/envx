package cli

import (
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	keypairUsage = "keypair [command]"
	keypairShort = "Manage a secret group's keypair"
	keypairLong  = `
		Generate, inspect, or rotate the asymmetric keypair for one secret group.
		Managed public keys are stored in secrets.yaml; private keys are stored in the
		configured git-ignored envx.keys file.
	`
)

// NewKeypairCmd builds the "keypair" parent command and registers its subcommands.
func NewKeypairCmd(factory Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   keypairUsage,
		Short: keypairShort,
		Long:  str.Dedent(keypairLong),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newKeypairGenerateCommand(factory))
	cmd.AddCommand(newKeypairInspectCommand(factory))
	cmd.AddCommand(newKeypairRotateCommand(factory))
	return cmd
}
