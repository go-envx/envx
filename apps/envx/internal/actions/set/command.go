package set

import (
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/go-envx/envx/apps/envx/internal/schema"
	"github.com/go-envx/envx/apps/envx/internal/str"
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
		exactly (e.g. "env/postgres", "apps/api-core/env/api-core").

		The target environment is determined by the --env flag, the ENVX_ENV env
		var, a manifest env setting, or defaults to "development".
	`
	example = `
		envx set env/postgres password insecure-password
		envx set env/postgres credentials.password s3cret --env=staging
		envx set env/gateway timeout 10 --env=production
	`
)

// -------------------------------------------------------------------------------------

// NewCommand builds the "set" command, which parses args into the action's
// params/config and executes the action.
func NewCommand(configPath *string) *cobra.Command {
	var cfg actionConfig

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ConfigPath = configPath
			cfg.Changed = cmd.Flags()

			// map args to action params
			p := actionParams{
				IncludePath: args[0],
				Key:         args[1],
				Value:       args[2],
			}

			// execute the action
			return execute(p, &cfg)
		},
	}

	flags.BindString(cmd, &cfg.Settings.Env, &schema.Env)
	return cmd
}
