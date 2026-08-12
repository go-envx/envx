package get

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "get <group> <key>"
	short = "Decrypt and print one stored secret"
	long  = `
		Get decrypts one secret from the workspace store and prints its plaintext
		to stdout. It requires an available private key for the group and fails
		when no key is available.
	`
	example = `
		envx secrets get production database_password
		DB_PASSWORD=$(envx secrets get production database_password)
	`
)

// NewCommand builds the command that decrypts and prints one secret value.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// map args to action params
			p := actionParams{
				Group: args[0],
				Key:   args[1],
			}

			// load command flags
			in := flags.GetInput(cmd.Flags())

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
