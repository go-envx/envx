package rotate

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "rotate <group>"
	short = "Rotate a secret-group keypair"
	long  = `
		Replace GROUP's keypair and re-encrypt every value in the group under the new
		public key. The current private key must be available so existing values can
		be decrypted and re-encrypted. The new public key is written to secrets.yaml
		and the new private key to the configured private-key file; when the current
		key comes from a higher-priority environment source, rotation into the local
		key file is refused. Private-key material is never printed.
	`
	example = `
		envx keypair rotate production
		envx keypair rotate shared
	`
)

// NewCommand builds the keypair rotation command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// map args to action params
			p := actionParams{Group: args[0]}

			// load command flags
			flagset := cmd.Flags()
			in := flags.GetInput(flagset)

			// execute the action
			result, err := execute(p, in)
			if err != nil {
				return err
			}

			// render the result through the shared printer
			pr := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})
			return render(&renderParams{
				Printer: pr,
				Result:  result,
			})
		},
	}
	return cmd
}
