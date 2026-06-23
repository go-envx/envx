package get

import (
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/go-envx/envx/apps/envx/internal/schema"
	"github.com/go-envx/envx/apps/envx/internal/str"
	"github.com/spf13/cobra"
)

const (
	usage = "get <project> <key>"
	short = "Get the value of an environment variable for a project"
	long  = `
		Get resolves the merged environment for a project and prints the value
		of the specified key. The key is matched case-insensitively (uppercased).

		The target environment is determined by the --env flag, the ENVX_ENV env
		var, a manifest env setting, or defaults to "development".
	`
	example = `
		envx get api-core POSTGRES_HOST
		envx get api-core postgres_host --env=production
	`
)

// -------------------------------------------------------------------------------------

// NewCommand builds the "get" command, which parses args into the action's
// params/config, executes the action, and writes the value to stdout.
func NewCommand(configPath *string) *cobra.Command {
	var cfg actionConfig

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ConfigPath = configPath
			cfg.Changed = cmd.Flags()

			// map args to action params
			p := actionParams{
				Project: args[0],
				Key:     args[1],
			}

			// execute the action
			res, err := execute(p, &cfg)
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

	flags.BindBool(cmd, &cfg.Settings.Strict, &schema.Strict)
	flags.BindString(cmd, &cfg.Settings.Prefix, &schema.Prefix)
	flags.BindString(cmd, &cfg.Settings.Suffix, &schema.Suffix)
	flags.BindBool(cmd, &cfg.Settings.NamespacePrefix, &schema.NamespacePrefix)
	flags.BindString(cmd, &cfg.Settings.Env, &schema.Env)
	return cmd
}
