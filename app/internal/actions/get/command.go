package get

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "get <project> <key>"
	short = "Get the value of an environment variable for a project"
	long  = `
		Get resolves the merged environment for a project and prints the value
		of the specified key. The key is matched case-insensitively (uppercased).

		The target environment is determined by the --env flag, the ENVX_ENV env
		var, a manifest env setting, or defaults to the first environment declared
		in envx.yaml.
	`
	example = `
		envx get api-service DATABASE_HOST
		envx get api-service database_host --env=production
	`
)

// -------------------------------------------------------------------------------------

// NewCommand builds the "get" command, which parses args into the action's
// params/config, executes the action, and writes the value to stdout.
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
				Project: args[0],
				Key:     args[1],
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
	return cmd
}
