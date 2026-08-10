package set

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "set <include-path> <key> <value>"
	short = "Set an environment variable in a namespace's overlay file"
	long  = `
		Set writes a key/value pair to the environment overlay file for the
		given include path. The key supports dot notation for nested YAML paths
		(e.g. "credentials.password").

		The include path must match an entry from a project's includes list
		exactly (e.g. "env/database", "api-service/env/values").

		The target environment is determined by the --env flag, the ENVX_ENV env
		var, a manifest env setting, or defaults to the first environment declared
		in envx.yaml.
	`
	example = `
		envx set api-service/env/values log_level warn --env=production
		envx set env/database database.password rotated --env=production
		envx set env/gateway gateway.timeout 10
	`
)

// NewCommand builds the "set" command, which parses args into the action's
// params/config and executes the action.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			// map args to action params
			p := actionParams{
				IncludePath: args[0],
				Key:         args[1],
				Value:       args[2],
			}

			// execute the action
			in := flags.GetInput(cmd.Flags())
			return execute(p, in)
		},
	}

	flags.Register(cmd.Flags(), flags.WithEnv)
	return cmd
}
