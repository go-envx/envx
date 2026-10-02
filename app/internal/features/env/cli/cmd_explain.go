package cli

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/arg"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	explainUsage = "explain <project> [key]"
	explainShort = "Show where each resolved value came from"
	explainLong  = `
		Explain resolves a project's environment and reports, for each key, its
		type, literal value, the file it was resolved from, and a resolution
		status. It never aborts on a failed value: an unresolved key is reported
		through its status and the command still exits 0. With no key it explains
		every key; with a key it explains just that one.

		Secret references are classified without materializing plaintext by
		default; pass --reveal to add a RESOLVED column with their decrypted
		values. Source paths are shown relative to envx.yaml unless --absolute is
		set. Use --output=json for machine-readable output.
	`
	explainExample = `
		envx explain api-service
		envx explain api-service DATABASE_HOST
		envx explain api-service --reveal
		envx explain api-service --absolute
		envx explain api-service --output=json
	`
)

// NewExplainCommand builds the "explain" command.
func NewExplainCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     explainUsage,
		Short:   explainShort,
		Long:    str.Dedent(explainLong),
		Example: str.Dedent(explainExample, 2),
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract command-line arguments.
			project := args[0]
			key := arg.Optional(args, 1)

			// Extract command-line flag values.
			fs := cmd.Flags()
			configPath := flags.Config.Get(fs)
			reveal := revealFlag.Get(fs)
			absolute := absoluteFlag.Get(fs)
			output := flags.Output.Get(fs)
			envOptions := getEnvOptions(fs)

			// Obtain the environment service using the configuration path.
			envService, err := f.EnvService(configPath)
			if err != nil {
				return err
			}

			// Perform the environment explain operation.
			result, err := envService.Explain(env.ExplainParams{
				Project: project,
				Key:     key,
				Reveal:  reveal,
				Options: envOptions,
			})
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the result of the environment explain operation.
			return outputExplain(console, result, output, reveal, absolute)
		},
	}

	// Bind the command-line flags.
	{
		fs := cmd.Flags()
		flags.Bind(fs, &flags.Output)
		flags.Bind(fs, &revealFlag)
		flags.Bind(fs, &absoluteFlag)
		bindEnvOptionsFlags(fs)
	}

	return cmd
}

// explainRenderer renders an ExplainResult to the console.
type explainRenderer struct {
	console  *printer.Printer
	reveal   bool
	absolute bool
}

// outputExplain renders the explanation in table or JSON format.
func outputExplain(
	console *printer.Printer,
	result *env.ExplainResult,
	format string,
	reveal bool,
	absolute bool,
) error {
	render := explainRenderer{
		console: console,
		reveal: reveal,
		absolute: absolute,
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
