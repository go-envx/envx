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
	generateUsage = "generate <group>"
	generateShort = "Generate a missing secret-group keypair"
	generateLong  = `
		Generate a keypair for GROUP and write its public key to secrets.yaml and
		its private key to the configured private-key file. Existing groups are
		rejected; use 'envx keypair rotate' to replace an existing keypair.
	`
	generateExample = `
		envx keypair generate production
		envx keypair generate shared
	`
)

// newKeypairGenerateCommand builds the keypair generation command.
func newKeypairGenerateCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     generateUsage,
		Short:   generateShort,
		Long:    str.Dedent(generateLong),
		Example: str.Dedent(generateExample, 2),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract command-line argument for group.
			group := args[0]

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

			// Generate the keypair using the secrets service.
			result, err := secretsService.GenerateKeypair(group)
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the result of the keypair generation.
			return outputKeypairGenerate(
				console,
				result,
			)
		},
	}
	return cmd
}

// outputKeypairGenerate reports the new keypair metadata and target paths without
// printing private-key material.
func outputKeypairGenerate(
	console *printer.Printer,
	result secrets.GenerateKeypairResult,
) error {
	return console.LogMessage(fmt.Sprintf(
		"Generated keypair for group %q:\n"+
			"  public key: %s\n"+
			"  secrets store: %s\n"+
			"  private key file: %s",
		result.Keypair.Group,
		result.Keypair.PublicKey,
		str.QuotePath(result.PublicKeyLocation),
		str.QuotePath(result.PrivateKeyLocation),
	))
}
