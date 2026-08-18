package inspect

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "inspect <group>"
	short = "Inspect a secret-group keypair"
	long  = `
		Inspect GROUP without writing files or prompting. The private-key status is
		reported as not_available, valid, or invalid; private-key material is never
		printed.
	`
	example = `
		envx keypair inspect production
	`
)

// NewCommand builds the keypair inspection command.
func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// map args to action params
			p := actionParams{
				Group: args[0],
			}

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
}
