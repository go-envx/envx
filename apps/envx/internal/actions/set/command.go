// Package set implements "envx set <include-path> <key> <value>": write a
// key/value into an environment overlay file. Its only export is NewCommand.
package set

import (
	"github.com/go-envx/envx/apps/envx/internal/actions"
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/shared/str"
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
// NewCommand builds the "set" command. It parses the three positional args and
// delegates to the shell, which resolves the target overlay without a project
// (set never merges an environment). g is the shared root context.
func NewCommand(g *config.Global) *cobra.Command {
	var cfg actionConfig

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Global = g
			cfg.Changed = cmd.Flags()
			return execute(actionParams{
				IncludePath: args[0],
				Key:         args[1],
				Value:       args[2],
			}, cfg)
		},
	}

	actions.RegisterEnvFlag(cmd, &cfg.Env)
	return cmd
}
