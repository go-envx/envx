package delete

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "delete <group> <key>"
	short = "Remove one stored secret value"
	long  = `
		Delete removes one secret from the workspace store. The group's public key
		and its remaining values are preserved, so deleting a value never tears
		down the group identity. Deleting a value that does not exist is an error.
	`
	example = `
		envx secrets delete production database_password
		envx secrets delete shared service_token
	`
)

// NewCommand builds the command that removes one stored secret value.
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
