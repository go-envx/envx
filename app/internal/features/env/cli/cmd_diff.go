package cli

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	diffUsage = "diff <project> <env-a> <env-b>"
	diffShort = "Compare a project's resolved environment across two environments"
	diffLong  = `
		Diff resolves the same project under two environments and reports the
		differences: keys added, removed, or changed between env-a and env-b.

		Secret references and {{VAR}} substitutions are compared as declarations
		(e.g. "secret://group/key") without resolution, so changing a reference is
		visible even when both references currently resolve to the same value. Pass
		--reveal to resolve and substitute each side and compare the resulting
		values instead. Use --output=json for machine-readable output.
	`
	diffExample = `
		envx diff api-service development production
		envx diff api-service development production --reveal
		envx diff api-service development production --output=json
	`
)

// NewDiffCommand builds the "diff" command.
func NewDiffCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     diffUsage,
		Short:   diffShort,
		Long:    str.Dedent(diffLong),
		Example: str.Dedent(diffExample, 2),
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract command-line arguments.
			project := args[0]
			envA := args[1]
			envB := args[2]

			// Extract command-line flag values.
			fs := cmd.Flags()
			configPath := flags.Config.Get(fs)
			reveal := revealFlag.Get(fs)
			output := flags.Output.Get(fs)
			envOptions := getEnvOptions(fs)

			// Obtain the environment service using the configuration path.
			envService, err := f.EnvService(configPath)
			if err != nil {
				return err
			}

			// Perform the environment diff operation.
			result, err := envService.Diff(env.DiffParams{
				Project:      project,
				EnvironmentA: envA,
				EnvironmentB: envB,
				Reveal:       reveal,
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

			// Output the result of the environment diff operation.
			return outputDiff(console, result, output)
		},
	}

	// Bind the command-line flags.
	{
		fs := cmd.Flags()
		flags.Bind(fs, &flags.Output)
		flags.Bind(fs, &revealFlag)
		bindEnvOptionsFlags(fs, without(&envFlag))
	}

	return cmd
}

// diffRenderer renders a DiffResult to the console.
type diffRenderer struct {
	console *printer.Printer
}

// outputDiff renders the environment comparison in table or JSON format.
func outputDiff(
	console *printer.Printer,
	result *env.DiffResult,
	format string,
) error {
	render := diffRenderer{
		console: console,
	}

	switch format {
	case "", "table":
		return render.table(result)
	case "json":
		return render.json(result)
	default:
		return fmt.Errorf("invalid output format %q (want table or json)", format)
	}
}
