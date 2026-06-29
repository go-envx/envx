package get

import (
	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/schema"
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
			// map args to action params
			p := actionParams{
				Project: args[0],
				Key:     args[1],
			}

			// execute the action
			in := cfg.input(cmd, configPath)
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

	flags.BindBool(cmd, &cfg.Strict, &schema.Strict)
	flags.BindString(cmd, &cfg.Prefix, &schema.Prefix)
	flags.BindString(cmd, &cfg.Suffix, &schema.Suffix)
	flags.BindBool(cmd, &cfg.NamespacePrefix, &schema.NamespacePrefix)
	flags.BindString(cmd, &cfg.Env, &schema.Env)
	return cmd
}

// -------------------------------------------------------------------------------------

// input gathers the explicitly-set flags into a *config.Input for resolution,
// marking each setting present only when the user changed it on the command line.
func (c *actionConfig) input(cmd *cobra.Command, configPath *string) *config.Input {
	return &config.Input{
		ConfigPath:      configPath,
		Env:             flags.OptionalString(cmd, &schema.Env, c.Env),
		Strict:          flags.OptionalBool(cmd, &schema.Strict, c.Strict),
		Prefix:          flags.OptionalString(cmd, &schema.Prefix, c.Prefix),
		Suffix:          flags.OptionalString(cmd, &schema.Suffix, c.Suffix),
		NamespacePrefix: flags.OptionalBool(cmd, &schema.NamespacePrefix, c.NamespacePrefix),
	}
}
