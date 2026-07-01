package explain

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/arg"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "explain <project> [key]"
	short = "Show where each resolved value came from"
	long  = `
		Explain resolves a project's environment and reports, for each key, the
		value and the file it was resolved from. With no key it explains every
		key; with a key it explains just that one.

		Values are masked by default; pass --reveal to print them in plaintext.
		Use --output=json for machine-readable output.
	`
	example = `
		envx explain api-core
		envx explain api-core POSTGRES_HOST --reveal
		envx explain api-core --output=json
	`
)

// -------------------------------------------------------------------------------------

// NewCommand builds the "explain" command, which parses args into the action's
// params/config, executes the action, and renders the result in the specified format.
// It accepts a project and an optional key. If the key is present it explains just
// that key. If the key is absent it explains all keys.
func NewCommand() *cobra.Command {
	var (
		reveal bool
		output string
	)

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// map args to action params
			p := actionParams{
				Project: args[0],
				Key:     arg.Optional(args, 1),
			}

			// execute the action
			in := flags.GetInput(cmd.Flags())
			res, err := execute(p, in)
			if err != nil {
				return err
			}

			// render the result
			return render(&renderParams{
				Writer: cmd.OutOrStdout(),
				Result: res,
				Format: output,
				Reveal: reveal,
			})
		},
	}

	flags.Register(cmd.Flags(),
		flags.WithEnv,
		flags.WithStrict,
		flags.WithPrefix,
		flags.WithSuffix,
		flags.WithNamespacePrefix,
	)
	flags.BindBool(cmd.Flags(), &reveal, &schema.Reveal)
	flags.BindString(cmd.Flags(), &output, &schema.Output)
	return cmd
}
