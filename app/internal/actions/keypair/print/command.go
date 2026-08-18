package print

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "print"
	short = "Print an unassigned keypair"
	long  = `
		Generate an unassigned keypair with the configured cipher. The keypair is
		written only to standard output and no workspace files are changed. Use
		--cipher to override the workspace configuration.
	`
	example = `
		envx keypair print
		envx keypair print --cipher nacl-box
	`
)

// NewCommand builds the ephemeral keypair print command.
func NewCommand() *cobra.Command {
	var cipherName string

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// load command flags
			flagset := cmd.Flags()
			in := flags.GetInput(flagset)

			// execute the action
			result, err := execute(in, cipherName)
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

	flags.BindString(cmd.Flags(), &cipherName, &schema.Cipher)

	return cmd
}
