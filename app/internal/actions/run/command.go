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
		var, a manifest env setting, or defaults to the first environment declared
		in envx.yaml.
	`
	example = `
		envx run api-service -- npm start
		envx run api-service --env=production -- node server.js
		envx run api-service --overload -- ./run.sh
	`
)

// NewCommand builds the "run" command, which parses args into the action's
// params/config, executes the action, and runs the specified command with the
// merged environment for a project.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    validateArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// validateArgs guarantees exactly one project before "--", so args[0]
			// is the project and args[1:] is the command to run.
			p := actionParams{
				Project:  args[0],
				ExecArgs: args[1:],
			}

			// get the flag inputs
			flagset := cmd.Flags()
			input := flags.GetInput(flagset)

			// execute the action
			return execute(p, input, streams{
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
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
		flags.WithOverload,
	)

	return cmd
}

// validateArgs enforces run's positional layout: exactly one project name, a
// "--" separator, then at least one command word. Validating here (rather than in
// RunE) makes a malformed invocation a usage error, so Cobra prints the help text
// and envx exits with the usage code.
func validateArgs(cmd *cobra.Command, args []string) error {
	dash := cmd.ArgsLenAtDash()
	switch {
	case dash < 0:
		return errors.New("missing '--' separator before the command to run")
	case dash != 1:
		return fmt.Errorf("run accepts exactly one project before '--', got %d", dash)
	case len(args) == dash:
		return errors.New("no command specified after '--'")
	}
	return nil
}
