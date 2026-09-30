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
	decryptUsage = "decrypt"
	decryptShort = "Decrypt stored values into plaintext"
	decryptLong  = `
		Decrypt rewrites encrypted values in the workspace store as plaintext in
		place, using each group's private key. Values that are already plaintext are
		left untouched, so the command is safe to run repeatedly. Use --group and
		--key to narrow the operation; by default every encrypted value is decrypted.

		A group whose private key is unavailable is skipped with a warning, so the
		groups you can decrypt still succeed. A --group or --key that matches no
		stored value is an error.

		Decryption writes plaintext secrets to disk. Re-encrypt with
		'envx secrets encrypt' before committing the store.
	`
	decryptExample = `
		envx secrets decrypt
		envx secrets decrypt --group production
		envx secrets decrypt --group shared --key service_token
	`
)

// newSecretsDecryptCommand builds the command that decrypts stored values in place.
func newSecretsDecryptCommand(f Factory) *cobra.Command {
	var (
		group, key string
		verbose    bool
	)

	cmd := &cobra.Command{
		Use:     decryptUsage,
		Short:   decryptShort,
		Long:    str.Dedent(decryptLong),
		Example: str.Dedent(decryptExample, 2),
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

			// Decrypt the stored secrets using the secrets service.
			result, err := secretsService.DecryptSecrets(group, key)
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the result of the secret decryption operation.
			return outputSecretsDecrypt(console, result, verbose)
		},
	}

	flags.Bind(cmd.Flags(), &group, &groupFlag)
	flags.Bind(cmd.Flags(), &key, &keyFlag)
	flags.Bind(cmd.Flags(), &verbose, &flags.Verbose)

	return cmd
}

// outputSecretsDecrypt reports any skipped-group warnings on stderr first, then the
// decrypted identities on stdout, never a secret value.
func outputSecretsDecrypt(
	console *printer.Printer,
	result secrets.DecryptSecretsResult,
	verbose bool,
) error {
	// Report groups skipped because no private key was available.
	for _, group := range result.UnavailableGroups {
		if err := console.LogWarning(fmt.Sprintf(
			"no private key available for group %q; its secrets were left encrypted",
			group,
		)); err != nil {
			return err
		}
	}

	// Separate unavailable groups from decrypted secrets with a blank line if both exist.
	if len(result.UnavailableGroups) > 0 && len(result.Secrets) > 0 {
		if err := console.LogBlank(); err != nil {
			return err
		}
	}

	// Report summary count and store location.
	switch {
	case len(result.Secrets) > 0:
		var b strings.Builder
		fmt.Fprintf(
			&b,
			"Decrypted %s in:\n%s",
			str.Pluralize(len(result.Secrets), "secret", "secrets"),
			str.QuotePath(result.Location),
		)
		if verbose {
			for _, secret := range result.Secrets {
				fmt.Fprintf(&b, "\n  %s/%s", secret.Group, secret.Key)
			}
		}
		return console.LogMessage(b.String())
	case len(result.UnavailableGroups) == 0:
		return console.LogMessage("No encrypted values to decrypt.")
	default:
		return nil
	}
}
