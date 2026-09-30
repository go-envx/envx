package cli

import (
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	secretsUsage = "secrets [command]"
	secretsShort = "Manage workspace secrets"
	//nolint:gosec // CLI command help text, not credentials.
	secretsLong = `
		Manage workspace secret keypairs and encrypted values.
	`
)

// NewSecretsCommand builds the "secrets" command and its management subcommands.
func NewSecretsCommand(factory Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   secretsUsage,
		Short: secretsShort,
		Long:  str.Dedent(secretsLong),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newSecretsGetCommand(factory))
	cmd.AddCommand(newSecretsSetCommand(factory))
	cmd.AddCommand(newSecretsEncryptCommand(factory))
	cmd.AddCommand(newSecretsDecryptCommand(factory))
	cmd.AddCommand(newSecretsDeleteCommand(factory))
	return cmd
}
