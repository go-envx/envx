package run

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "run <project> -- <command> [args...]"
	short = "Run a command with the merged environment for a project"
	long  = `
		Run executes a command with environment variables loaded from the
		project's namespace chain. Variables are merged in declaration order with
		later values winning.

		By default existing OS environment variables take precedence over file
		values; use --overload to let file values win instead.

		The target environment is determined by the --env flag, the ENVX_ENV env
		var, a manifest env setting, or defaults to "development".
	`
	example = `
		envx run api-core -- npm start
		envx run api-core --env=production -- node server.js
		envx run api-core --strict -- ./run.sh
	`
)

// -------------------------------------------------------------------------------------

// NewCommand builds the "run" command, which parses args into the action's
// params/config, executes the action, and runs the specified command with the
// merged environment for a project.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate args
			dashIdx := cmd.ArgsLenAtDash()
			if dashIdx < 1 {
				return fmt.Errorf("usage: %s", usage)
			}
			childArgs := args[dashIdx:]
			if len(childArgs) == 0 {
				return errors.New("no command specified after --")
			}

			// map args to action params
			p := actionParams{
				Project:  args[0],
				ExecArgs: childArgs,
			}

			// execute the action
			in := flags.GetInput(cmd.Flags())
			return execute(cmd.Context(), p, in, streams{
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
		},
	}

	flags.Register(cmd.Flags(),
		flags.WithEnv,
		flags.WithStrict,
		flags.WithPrefix,
		flags.WithSuffix,
		flags.WithNamespacePrefix,
		flags.WithOverload,
	)
	return cmd
}
