package secrets

import (
	"github.com/go-envx/envx/app/internal/actions/secrets/decrypt"
	"github.com/go-envx/envx/app/internal/actions/secrets/delete"
	"github.com/go-envx/envx/app/internal/actions/secrets/encrypt"
	"github.com/go-envx/envx/app/internal/actions/secrets/get"
	"github.com/go-envx/envx/app/internal/actions/secrets/set"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "secrets [command]"
	short = "Manage workspace secrets"
	long  = `
		Manage workspace secret keypairs and encrypted values.
	`
)

// NewCommand builds the "secrets" command and its management subcommands.
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
	cmd.AddCommand(get.NewCommand())
	cmd.AddCommand(set.NewCommand())
	cmd.AddCommand(encrypt.NewCommand())
	cmd.AddCommand(decrypt.NewCommand())
	cmd.AddCommand(delete.NewCommand())
	return cmd
}
