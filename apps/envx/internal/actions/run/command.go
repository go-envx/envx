package run

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/apps/envx/internal/flags"
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
// NewCommand builds the "run" command. It splits args at "--" (a project before
// it, the child command after), registers the engine-setting flags plus --env
// and the run-local --overload, and delegates to the shell. configPath points at
// the persistent --config flag.
func NewCommand(configPath *string) *cobra.Command {
	var cfg actionConfig

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dashIdx := cmd.ArgsLenAtDash()
			if dashIdx < 1 {
				return fmt.Errorf("usage: %s", usage)
			}
			project := args[:dashIdx]
			childArgs := args[dashIdx:]
			if len(childArgs) == 0 {
				return errors.New("no command specified after --")
			}

			cfg.ConfigPath = configPath
			cfg.Changed = cmd.Flags()

			return execute(cmd.Context(), actionParams{
				Project:  project[0],
				ExecArgs: childArgs,
				Stdout:   cmd.OutOrStdout(),
				Stderr:   cmd.ErrOrStderr(),
			}, &cfg)
		},
	}

	flags.NewStrictFlag(cmd, &cfg.Settings.Strict)
	flags.NewPrefixFlag(cmd, &cfg.Settings.Prefix)
	flags.NewSuffixFlag(cmd, &cfg.Settings.Suffix)
	flags.NewNamespacePrefixFlag(cmd, &cfg.Settings.NamespacePrefix)
	flags.NewEnvFlag(cmd, &cfg.Settings.Env)
	flags.NewOverloadFlag(cmd, &cfg.Overload)
	return cmd
}
