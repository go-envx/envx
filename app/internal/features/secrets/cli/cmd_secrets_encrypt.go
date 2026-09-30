package cli

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	encryptUsage = "encrypt"
	encryptShort = "Encrypt plaintext values in the store"
	encryptLong  = `
		Encrypt re-encrypts plaintext values in the workspace store in place using
		each group's public key. Values that are already encrypted are left
		untouched, so the command is safe to run repeatedly. Use --group and --key
		to narrow the operation; by default every plaintext value is encrypted. A
		--group or --key that matches no stored value is an error.
	`
	encryptExample = `
		envx secrets encrypt
		envx secrets encrypt --group production
		envx secrets encrypt --group shared --key service_token
	`
)

// newSecretsEncryptCommand builds the command that encrypts plaintext store
// values in place.
func newSecretsEncryptCommand(f Factory) *cobra.Command {
	var (
		group, key string
		verbose    bool
	)

	cmd := &cobra.Command{
		Use:     encryptUsage,
		Short:   encryptShort,
		Long:    str.Dedent(encryptLong),
		Example: str.Dedent(encryptExample, 2),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
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

			// Encrypt the secrets using the secrets service.
			result, err := secretsService.EncryptSecrets(group, key)
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the result of the secret encryption operation.
			return outputSecretsEncrypt(console, result, verbose)
		},
	}

	flags.Bind(cmd.Flags(), &group, &groupFlag)
	flags.Bind(cmd.Flags(), &key, &keyFlag)
	flags.Bind(cmd.Flags(), &verbose, &flags.Verbose)

	return cmd
}

// outputSecretsEncrypt reports how many values were encrypted and where, never a secret
// value. It lists each changed identity only when Verbose is set, and reports
// plainly when nothing needed encrypting.
func outputSecretsEncrypt(
	console *printer.Printer,
	result secrets.EncryptSecretsResult,
	verbose bool,
) error {
	switch {
	case len(result.Secrets) > 0:
		var b strings.Builder
		fmt.Fprintf(
			&b,
			"Encrypted %s in:\n%s",
			str.Pluralize(len(result.Secrets), "secret", "secrets"),
			str.QuotePath(result.Location),
		)
		if verbose {
			for _, secret := range result.Secrets {
				fmt.Fprintf(&b, "\n  %s/%s", secret.Group, secret.Key)
			}
		}
		return console.LogMessage(b.String())
	default:
		return console.LogMessage("No plaintext values to encrypt.")
	}
}
