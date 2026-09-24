package cli

import (
	"github.com/go-envx/envx/app/internal/features/secrets/cli/decrypt"
	"github.com/go-envx/envx/app/internal/features/secrets/cli/delete"
	"github.com/go-envx/envx/app/internal/features/secrets/cli/encrypt"
	"github.com/go-envx/envx/app/internal/features/secrets/cli/get"
	"github.com/go-envx/envx/app/internal/features/secrets/cli/set"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	secretsUsage = "secrets [command]"
	secretsShort = "Manage workspace secrets"
	//nolint:gosec // G101: CLI command help text, not credentials.
	secretsLong = `
		Manage workspace secret keypairs and encrypted values.
	`
)

// NewSecretsCmd builds the "secrets" command and its management subcommands.
func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   secretsUsage,
		Short: secretsShort,
		Long:  str.Dedent(secretsLong),
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
