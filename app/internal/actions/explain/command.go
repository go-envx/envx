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

		Secret references are masked as "secret://group/key" by default; pass
		--reveal to decrypt and show their plaintext. Use --output=json for
		machine-readable output.
	`
	example = `
		envx explain api-service
		envx explain api-service DATABASE_HOST
		envx explain api-service --reveal
		envx explain api-service --output=json
	`
)

// NewCommand builds the "explain" command, which parses args into the action's
// params/config, executes the action, and renders the result in the specified format.
// It accepts a project and an optional key. If the key is present it explains just
// that key. If the key is absent it explains all keys.
func NewCommand() *cobra.Command {
	var output string
	var reveal bool

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
				Reveal:  reveal,
			}

			// get the flag inputs
			flagset := cmd.Flags()
			input := flags.GetInput(flagset)

			// execute the action
			res, err := execute(p, input)
			if err != nil {
				return err
			}

			// render the result
			return render(&renderParams{
				Writer: cmd.OutOrStdout(),
				Result: res,
				Format: output,
			})
		},
	}

	flags.Register(cmd.Flags(),
		flags.WithEnv,
		flags.WithRequireOverlays,
		flags.WithPrefix,
		flags.WithSuffix,
		flags.WithDelimiter,
		flags.WithNamespacePrefix,
	)

	flags.BindString(cmd.Flags(), &output, &schema.Output)
	flags.BindBool(cmd.Flags(), &reveal, &schema.Reveal)

	return cmd
}
