package set

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "set <group> <key>"
	short = "Encrypt and store one secret value"
	long  = `
		Set adds or updates one secret in the workspace store. The plaintext is
		read from stdin, or from a hidden terminal prompt when stdin is interactive;
		it is never accepted as an argument or printed.
	`
	example = `
		printf '%s' "$DB_PASSWORD" | envx secrets set production database_password
		envx secrets set shared service_token
	`
)

// -------------------------------------------------------------------------------------

// NewCommand builds the command that securely enters one secret value.
func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// map args to action p
			p := actionParams{
				Group: args[0],
				Key:   args[1],
			}

			// load command flags
			flagset := cmd.Flags()
			in := flags.GetInput(flagset)

			// execute the action
			result, err := execute(p, in, streams{
				Stdin:  cmd.InOrStdin(),
				Stderr: cmd.ErrOrStderr(),
			})
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
}
