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
	setUsage = "set <group> <key> [plaintext]"
	setShort = "Encrypt and store one secret value"
	setLong  = `
		Set adds or updates one secret in the workspace store. The plaintext is
		read from stdin, or from a hidden terminal prompt with a length confirmation
		when stdin is interactive. An optional plaintext argument is also supported
		for automation. Use --no-confirm to skip the interactive confirmation
		prompt.
	`
	setExample = `
		printf '%s' "$DB_PASSWORD" | envx secrets set production database_password
		envx secrets set shared service_token
	`
)

// newSecretsSetCommand builds the command that securely enters one secret value.
func newSecretsSetCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     setUsage,
		Short:   setShort,
		Long:    str.Dedent(setLong),
		Example: str.Dedent(setExample, 2),
		Args:    cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract command-line arguments.
			group := args[0]
			key := args[1]
			var plaintext *string
			if len(args) == 3 {
				plaintext = &args[2]
			}

			// Extract command-line flag values.
			fs := cmd.Flags()
			configPath := flags.Config.Get(fs)

			// Obtain the secrets service using the configuration path.
			secretsService, err := f.SecretsService(configPath)
			if err != nil {
				return err
			}

			// Define a function to read the secret, either from the provided
			// plaintext argument or interactively from the user.
			readSecret := func() (string, error) {
				if plaintext != nil {
					return *plaintext, nil
				}
				return newSecretPrompt(secretPromptParams{
					Stdin:     cmd.InOrStdin(),
					Stderr:    cmd.ErrOrStderr(),
					NoConfirm: noConfirmFlag.Get(fs),
				}).readSecret()
			}

			// Read the secret value using the defined function.
			result, err := secretsService.SetSecret(group, key, readSecret)
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the result of the secret storage operation.
			return outputSecretsSet(console, result)
		},
	}

	// Bind the command-line flags.
	{
		fs := cmd.Flags()
		flags.Bind(fs, &noConfirmFlag)
	}

	return cmd
}

// outputSecretsSet reports the stored identity without printing the secret value.
func outputSecretsSet(
	console *printer.Printer,
	result secrets.SetSecretResult,
) error {
	return console.LogMessage(fmt.Sprintf(
		"Stored secret %q in group %q at:\n%s",
		result.Secret.Key,
		result.Secret.Group,
		str.QuotePath(result.Location),
	))
}
