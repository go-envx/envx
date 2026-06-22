package get

import (
	"fmt"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/settings"
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
// NewCommand builds the "get" command. It is the only exported symbol: it parses
// args into the action's params/config, calls the shell, and writes the value to
// stdout. configPath points at the persistent --config flag.
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

			res, err := execute(actionParams{Project: args[0], Key: args[1]}, &cfg)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), res.Value)
			return err
		},
	}

	config.BindBool(cmd, &cfg.Settings.Strict, &settings.Strict)
	config.BindString(cmd, &cfg.Settings.Prefix, &settings.Prefix)
	config.BindString(cmd, &cfg.Settings.Suffix, &settings.Suffix)
	config.BindBool(cmd, &cfg.Settings.NamespacePrefix, &settings.NamespacePrefix)
	config.BindString(cmd, &cfg.Settings.Env, &settings.Env)
	return cmd
}
