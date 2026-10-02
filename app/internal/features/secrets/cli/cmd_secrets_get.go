package cli

import (
	"fmt"
	"io"

	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	getUsage = "get <group> <key>"
	getShort = "Decrypt and print one stored secret"
	getLong  = `
		Get decrypts one secret from the workspace store and prints its plaintext
		to stdout. It requires an available private key for the group and fails
		when no key is available.
	`
	getExample = `
		envx secrets get production database_password
		DB_PASSWORD=$(envx secrets get production database_password)
	`
)

// newSecretsGetCommand builds the command that decrypts and prints one secret value.
func newSecretsGetCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     getUsage,
		Short:   getShort,
		Long:    str.Dedent(getLong),
		Example: str.Dedent(getExample, 2),
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract command-line arguments for group and key.
			group := args[0]
			key := args[1]

			// Extract the configuration path from the command flags.
			fs := cmd.Flags()
			configPath := flags.Config.Get(fs)

			// Obtain the secrets service using the configuration path.
			secretsService, err := f.SecretsService(configPath)
			if err != nil {
				return err
			}

			// Retrieve the secret value from the secrets service.
			result, err := secretsService.GetSecret(group, key)
			if err != nil {
				return err
			}

			// Output the retrieved secret value to stdout.
			return outputSecretsGet(cmd.OutOrStdout(), result.Value)
		},
	}

	return cmd
}

// outputSecretsGet prints the decrypted plaintext followed by a newline so the value is
// convenient to pipe.
func outputSecretsGet(w io.Writer, value string) error {
	_, err := fmt.Fprintln(w, value)
	return err
}
