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
	rotateUsage = "rotate <group>"
	rotateShort = "Rotate a secret-group keypair"
	rotateLong  = `
		Replace GROUP's keypair and re-encrypt every value in the group under the new
		public key. The current private key must be available so existing values can
		be decrypted and re-encrypted. The new public key is written to secrets.yaml
		and the new private key to the configured private-key file; when the current
		key comes from a higher-priority environment source, rotation into the local
		key file is refused. Private-key material is never printed.
	`
	rotateExample = `
		envx keypair rotate production
		envx keypair rotate shared
	`
)

// newKeypairRotateCommand builds the keypair rotation command.
func newKeypairRotateCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     rotateUsage,
		Short:   rotateShort,
		Long:    str.Dedent(rotateLong),
		Example: str.Dedent(rotateExample, 2),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract command-line argument for group.
			group := args[0]

			// Extract the configuration path from the command flags.
			fs := cmd.Flags()
			configPath := flags.Config.Get(fs)

			// Obtain the secrets service using the configuration path.
			secretsService, err := f.SecretsService(configPath)
			if err != nil {
				return err
			}

			// Rotate the keypair using the secrets service.
			result, err := secretsService.RotateKeypair(group)
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the result of the keypair rotation.
			return outputKeypairRotate(console, result)
		},
	}
	return cmd
}

// outputKeypairRotate reports the re-encryption count and file locations without keys.
func outputKeypairRotate(
	console *printer.Printer,
	result secrets.RotateKeypairResult,
) error {
	return console.LogMessage(fmt.Sprintf(
		"Rotated keypair for group %q and re-encrypted %s:\n"+
			"  public key: %s\n"+
			"  secrets store: %s\n"+
			"  private key file: %s",
		result.Keypair.Group,
		str.Pluralize(len(result.Secrets), "secret", "secrets"),
		result.Keypair.PublicKey,
		str.QuotePath(result.PublicKeyLocation),
		str.QuotePath(result.PrivateKeyLocation),
	))
}
