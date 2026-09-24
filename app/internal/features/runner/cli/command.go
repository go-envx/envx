package cli

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/str"
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

		By default an unresolved value aborts the run; use --ignore-errors to warn
		on each unresolved value, omit it from the child environment, and start the
		process anyway.

		The target environment is determined by the --env flag, the ENVX_ENV env
		var, a manifest env setting, or defaults to the first environment declared
		in envx.yaml.
	`
	example = `
		envx run api-service -- npm start
		envx run api-service --env=production -- node server.js
		envx run api-service --overload -- ./run.sh
		envx run api-service --ignore-errors -- ./run.sh
	`
)

// NewRunCmd builds the "run" command, which parses args into the action's
// params/config, executes the action, and runs the specified command with the
// merged environment for a project.
func NewRunCmd() *cobra.Command {
	var ignoreErrors bool

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
				Project:      args[0],
				ExecArgs:     args[1:],
				IgnoreErrors: ignoreErrors,
			}

			// get the flag inputs
			flagset := cmd.Flags()
			input := core.GetInput(flagset)

			// execute the action
			return execute(p, input, streams{
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
		},
	}

	env.RegisterFlags(cmd.Flags(),
		env.WithEnv,
		env.WithRequireOverlays,
		env.WithPrefix,
		env.WithSuffix,
		env.WithDelimiter,
		env.WithOverload,
		env.WithReferencePattern,
	)

	// --ignore-errors is a command-local flag, not a precedence-resolved setting,
	// so it binds directly rather than through RegisterFlags.
	flags.BindBool(cmd.Flags(), &ignoreErrors, &IgnoreErrors)

	return cmd
}

// NewCommand is an alias for NewRunCmd.
func NewCommand() *cobra.Command {
	return NewRunCmd()
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
