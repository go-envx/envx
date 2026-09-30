package cli

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	deleteUsage = "delete <group> <key>"
	deleteShort = "Remove one stored secret value"
	deleteLong  = `
		Delete removes one secret from the workspace store. The group's public key
		and its remaining values are preserved, so deleting a value never tears
		down the group identity. Deleting a value that does not exist is an error.
	`
	deleteExample = `
		envx secrets delete production database_password
		envx secrets delete shared service_token
	`
)

// newSecretsDeleteCommand builds the command that removes one stored secret value.
func newSecretsDeleteCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     deleteUsage,
		Short:   deleteShort,
		Long:    str.Dedent(deleteLong),
		Example: str.Dedent(deleteExample, 2),
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract command-line arguments for group and key.
			group := args[0]
			key := args[1]

			// Extract the configuration path from the command flags.
			configPath, err := cmd.Flags().GetString(flags.Config.Name)
			if err != nil {
				return err
			}

			// Obtain the secrets service using the configuration path.
			secretsService, err := f.SecretsService(configPath)
			if err != nil {
				return err
			}

			// Delete the secret value using the secrets service.
			result, err := secretsService.DeleteSecret(group, key)
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the result of the secret deletion operation.
			return outputSecretsDelete(console, result)
		},
	}

	return cmd
}

// outputSecretsDelete reports the removed identity and the updated store location.
func outputSecretsDelete(
	console *printer.Printer,
	result secrets.DeleteSecretResult,
) error {
	return console.LogMessage(fmt.Sprintf(
		"Deleted secret %q from group %q in:\n%s",
		result.Secret.Key,
		result.Secret.Group,
		str.QuotePath(result.Location),
	))
}
