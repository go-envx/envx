package keypair

import (
	"github.com/go-envx/envx/app/internal/actions/keypair/generate"
	"github.com/go-envx/envx/app/internal/actions/keypair/inspect"
	"github.com/go-envx/envx/app/internal/actions/keypair/print"
	"github.com/go-envx/envx/app/internal/actions/keypair/rotate"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "keypair [command]"
	short = "Manage a secret group's keypair"
	long  = `
		Generate, inspect, or rotate the asymmetric keypair for one secret group, or
		print an unassigned pair without storing it. Managed public keys are stored in
		secrets.yaml; private keys are stored in the configured git-ignored envx.keys
		file.
	`
)

// NewCommand builds the "keypair" parent command and registers its subcommands.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   usage,
		Short: short,
		Long:  str.Dedent(long),
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
