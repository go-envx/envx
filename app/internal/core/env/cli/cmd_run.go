package cli

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/env"
	workspacecli "github.com/go-envx/envx/app/internal/core/workspace/cli"
	"github.com/go-envx/envx/app/internal/lib/printer"
	"github.com/go-envx/envx/app/internal/lib/stdio"
	"github.com/go-envx/envx/app/internal/lib/str"
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
			configPath := workspacecli.ConfigPath(cmd)
			ignoreErrors := ignoreErrorsFlag.Value(cmd)
			settings := workspacecli.Settings(cmd)

			// Obtain the environment service using the configuration path.
			envService, err := f.NewEnvService(configPath)
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Materialize the environment and run the child with exactly it. By
			// default a single unresolved value aborts before the child starts, so it
			// can never receive an unresolved reference as plaintext. Under
			// --ignore-errors each resolution failure is printed as a warning and its
			// key is omitted, so the child still starts and inherits the omitted key
			// from the ambient environment. Structural failures stay fatal in both
			// modes. A non-zero or signal-terminated child exit surfaces as an
			// *exitcode.Error so main.go can propagate it.
			return envService.Run(env.RunParams{
				Project: project,
				Args:    execArgs,
				Streams: stdio.Streams{
					In:     cmd.InOrStdin(),
					Out:    cmd.OutOrStdout(),
					ErrOut: cmd.ErrOrStderr(),
				},
				OnWarning: func(warning diagnostic.Finding) error {
					return outputRunWarning(console, warning)
				},
				IgnoreErrors: ignoreErrors,
				Options:      settings,
			})
		},
	}

	// Bind the command-line flags.
	ignoreErrorsFlag.Bind(cmd)
	workspacecli.BindSettings(cmd)

	return cmd
}

// outputRunWarning prints one downgraded resolution failure as a warning.
func outputRunWarning(console *printer.Printer, warning diagnostic.Finding) error {
	return console.LogWarning(warning.Message)
}
