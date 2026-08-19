package explain

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/arg"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "explain <project> [key]"
	short = "Show where each resolved value came from"
	long  = `
		Explain resolves a project's environment and reports, for each key, its
		type, literal value, the file it was resolved from, and a resolution
		status. It never aborts on a failed value: an unresolved key is reported
		through its status and the command still exits 0. With no key it explains
		every key; with a key it explains just that one.

		Secret references are classified without materializing plaintext by
		default; pass --reveal to add a RESOLVED column with their decrypted
		values. Source paths are shown relative to envx.yaml unless --absolute is
		set. Use --output=json for machine-readable output.
	`
	example = `
		envx explain api-service
		envx explain api-service DATABASE_HOST
		envx explain api-service --reveal
		envx explain api-service --absolute
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
	var absolute bool

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// map args to action params
			p := actionParams{
				Project:  args[0],
				Key:      arg.Optional(args, 1),
				Reveal:   reveal,
				Absolute: absolute,
			}

			// get the flag inputs
			flagset := cmd.Flags()
			input := flags.GetInput(flagset)

			// execute the action
			res, err := execute(p, input)
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
				Result:  res,
				Format:  output,
				Reveal:  reveal,
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
		flags.WithOverload,
	)

	flags.BindString(cmd.Flags(), &output, &schema.Output)
	flags.BindBool(cmd.Flags(), &reveal, &schema.Reveal)
	flags.BindBool(cmd.Flags(), &absolute, &schema.Absolute)

	return cmd
}
