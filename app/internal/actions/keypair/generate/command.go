package generate

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "generate <group>"
	short = "Generate a missing secret-group keypair"
	long  = `
		Generate a keypair for GROUP and write its public key to secrets.yaml and
		its private key to the configured private-key file. Existing groups are
		rejected; use keypair rotation when that workflow is available.
	`
	example = `
		envx keypair generate production
		envx keypair generate shared
	`
)

// NewCommand builds the keypair generation command.
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

			// render the result
			return render(&renderParams{
				Writer: cmd.OutOrStdout(),
				Result: result,
			})
		},
	}
	return cmd
}
