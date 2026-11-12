package cli

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/core/env"
	workspacecli "github.com/go-envx/envx/app/internal/core/workspace/cli"
	"github.com/go-envx/envx/app/internal/lib/printer"
	"github.com/go-envx/envx/app/internal/lib/str"
	"github.com/spf13/cobra"
)

const (
	diffUsage = "diff <project> <env-a> <env-b>"
	diffShort = "Compare a project's declared values across two environments"
	diffLong  = `
		Diff merges the same project under two environments and reports the
		differences: keys added, removed, or changed between env-a and env-b.

		Values are compared as declared. Secret references and {{VAR}}
		substitutions are never resolved, so changing a reference is visible even
		when both references resolve to the same value, and no secret is decrypted
		or fetched. Use --output=json for machine-readable output.
	`
	diffExample = `
		envx diff api-service development production
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
			configPath := workspacecli.ConfigPath(cmd)
			output := outputFlag.Value(cmd)
			settings := workspacecli.Settings(cmd)

			// Obtain the environment service using the configuration path.
			envService, err := f.NewEnvService(configPath)
			if err != nil {
				return err
			}

			// Perform the environment diff operation.
			result, err := envService.Diff(env.DiffParams{
				Project:      project,
				EnvironmentA: envA,
				EnvironmentB: envB,
				Options:      settings,
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
			return outputDiff(console, &result, output)
		},
	}

	// Bind the command-line flags.
	outputFlag.Bind(cmd)
	workspacecli.BindSettings(cmd, "env")

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
