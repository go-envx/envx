package cli

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/runner"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	runUsage = "run <project> -- <command> [args...]"
	runShort = "Run a command with the merged environment for a project"
	runLong  = `
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
	runExample = `
		envx run api-service -- npm start
		envx run api-service --env=production -- node server.js
		envx run api-service --overload -- ./run.sh
		envx run api-service --ignore-errors -- ./run.sh
	`
)

// NewRunCommand builds the "run" command.
func NewRunCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     runUsage,
		Short:   runShort,
		Long:    str.Dedent(runLong),
		Example: str.Dedent(runExample, 2),
		Args: func(cmd *cobra.Command, args []string) error {
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
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract command-line arguments. Args validation guarantees exactly one project
			// before "--", so args[0] is the project and args[1:] is the command to run.
			project := args[0]
			execArgs := args[1:]

			// Extract command-line flag values.
			fs := cmd.Flags()
			configPath := flags.Config.Get(fs)
			ignoreErrors := ignoreErrorsFlag.Get(fs)
			envOptions := getEnvOptions(fs)

			// Obtain the environment service using the configuration path.
			envService, err := f.EnvService(configPath)
			if err != nil {
				return err
			}

			// Obtain the runner service from the factory.
			runnerService, err := f.RunnerService()
			if err != nil {
				return err
			}

			// Materialize the complete environment. By default a single unresolved
			// value aborts here, before the child starts, so it can never receive an
			// unresolved reference as plaintext. Under --ignore-errors each
			// resolution failure is downgraded to a warning and its key is omitted,
			// so the child still starts and inherits the omitted key from the
			// ambient environment. Structural failures stay fatal in both modes.
			result, err := envService.Materialize(env.MaterializeParams{
				Project:      project,
				IgnoreErrors: ignoreErrors,
				Options:      envOptions,
			})
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output any downgraded resolution warnings.
			if err := outputRunWarnings(console, result.Warnings); err != nil {
				return err
			}

			// Run the child process with the ready environment. The runner injects it
			// verbatim, forwards signals to the child, and surfaces a non-zero or
			// signal-terminated exit as an *exitcode.Error so main.go can propagate
			// it.
			return runnerService.Run(runner.RunParams{
				Args:   execArgs,
				Env:    result.Environment.All(),
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
				Stdin:  cmd.InOrStdin(),
			})
		},
	}

	// Bind the command-line flags.
	{
		fs := cmd.Flags()
		flags.Bind(fs, &ignoreErrorsFlag)
		bindEnvOptionsFlags(fs)
	}

	return cmd
}

// outputRunWarnings prints each downgraded resolution failure as a warning.
func outputRunWarnings(console *printer.Printer, warnings []error) error {
	for _, warning := range warnings {
		if err := console.LogWarning(warning.Error()); err != nil {
			return err
		}
	}
	return nil
}
