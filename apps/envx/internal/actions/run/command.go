package run

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/go-envx/envx/apps/envx/internal/schema"
	"github.com/go-envx/envx/apps/envx/internal/str"
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
func NewCommand(configPath *string) *cobra.Command {
	var cfg actionConfig

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ConfigPath = configPath
			cfg.Changed = cmd.Flags()

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
			return execute(cmd.Context(), p, &cfg, streams{
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
		},
	}

	flags.BindBool(cmd, &cfg.Settings.Strict, &schema.Strict)
	flags.BindString(cmd, &cfg.Settings.Prefix, &schema.Prefix)
	flags.BindString(cmd, &cfg.Settings.Suffix, &schema.Suffix)
	flags.BindBool(cmd, &cfg.Settings.NamespacePrefix, &schema.NamespacePrefix)
	flags.BindString(cmd, &cfg.Settings.Env, &schema.Env)
	flags.BindBool(cmd, &cfg.Overload, &schema.Overload)
	return cmd
}
