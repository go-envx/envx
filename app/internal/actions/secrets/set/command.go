package set

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "set <group> <key> [plaintext]"
	short = "Encrypt and store one secret value"
	long  = `
		Set adds or updates one secret in the workspace store. The plaintext is
		read from stdin, or from a hidden terminal prompt with a length confirmation
		when stdin is interactive. An optional plaintext argument is also supported
		for automation. Use --no-confirm to skip the interactive confirmation
		prompt.
	`
	example = `
		printf '%s' "$DB_PASSWORD" | envx secrets set production database_password
		envx secrets set shared service_token
	`
)

// -------------------------------------------------------------------------------------

// NewCommand builds the command that securely enters one secret value.
func NewCommand() *cobra.Command {
	var noConfirm bool

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			// map args to action p
			p := actionParams{
				Group:     args[0],
				Key:       args[1],
				NoConfirm: noConfirm,
			}
			if len(args) == 3 {
				p.Plaintext = &args[2]
			}

			// load command flags
			flagset := cmd.Flags()
			in := flags.GetInput(flagset)

			// execute the action
			result, err := execute(p, in, readerParams{
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

	flags.BindBool(cmd.Flags(), &noConfirm, &schema.NoConfirm)

	return cmd
}
