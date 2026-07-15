package diff

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "diff <project> <env-a> <env-b>"
	short = "Compare a project's resolved environment across two environments"
	long  = `
		Diff resolves the same project under two environments and reports the
		differences: keys added, removed, or changed between env-a and env-b.

		Values are masked by default; pass --reveal to print them in plaintext.
		Use --output=json for machine-readable output.
	`
	example = `
		envx diff api-service development production
		envx diff api-service development production --reveal
		envx diff api-service development production --output=json
	`
)

// -------------------------------------------------------------------------------------

// NewCommand builds the "diff" command, which parses args into the action's
// params/config, executes the action, and renders the structured diff in the
// specified format.
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
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			// map args to action params
			p := actionParams{
				Project: args[0],
				EnvA:    args[1],
				EnvB:    args[2],
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
		flags.WithRequireOverlays,
		flags.WithPrefix,
		flags.WithSuffix,
		flags.WithDelimiter,
		flags.WithNamespacePrefix,
	)
	flags.BindBool(cmd.Flags(), &reveal, &schema.Reveal)
	flags.BindString(cmd.Flags(), &output, &schema.Output)
	return cmd
}
