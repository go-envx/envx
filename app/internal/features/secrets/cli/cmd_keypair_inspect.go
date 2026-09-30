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
	inspectUsage = "inspect <group>"
	inspectShort = "Inspect a secret-group keypair"
	inspectLong  = `
		Inspect GROUP without writing files or prompting. The private-key status is
		reported as not_available, valid, or invalid; private-key material is never
		printed.
	`
	inspectExample = `
		envx keypair inspect production
	`
)

// newKeypairInspectCommand builds the keypair inspection command.
func newKeypairInspectCommand(f Factory) *cobra.Command {
	return &cobra.Command{
		Use:     inspectUsage,
		Short:   inspectShort,
		Long:    str.Dedent(inspectLong),
		Example: str.Dedent(inspectExample, 2),
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

			// Inspect the keypair using the secrets service.
			result, err := secretsService.InspectKeypair(group)
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the keypair inspection results.
			return outputKeypairInspect(console, result)
		},
	}
}

// outputKeypairInspect reports public key and private-key status without key bytes.
func outputKeypairInspect(
	console *printer.Printer,
	result secrets.KeypairMetadata,
) error {
	return console.LogMessage(fmt.Sprintf(
		"Keypair for group %q:\n"+
			"  public key: %s\n"+
			"  private key: %s",
		result.Group,
		result.PublicKey,
		result.PrivateKeyStatus,
	))
}
