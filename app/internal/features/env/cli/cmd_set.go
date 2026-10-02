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
	setUsage = "set <include-path> <key> <value>"
	setShort = "Set an environment variable in a namespace's overlay file"
	setLong  = `
		Set writes a key/value pair to the environment overlay file for the
		given include path. The key supports dot notation for nested YAML paths
		(e.g. "credentials.password").

		The include path must match an entry from a project's includes list
		exactly (e.g. "env/database", "api-service/env/values").

		The target environment is determined by the --env flag, the ENVX_ENV env
		var, a manifest env setting, or defaults to the first environment declared
		in envx.yaml.
	`
	setExample = `
		envx set api-service/env/values log_level warn --env=production
		envx set env/database database.password rotated --env=production
		envx set env/gateway gateway.timeout 10
	`
)

// NewSetCommand builds the "set" command.
func NewSetCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     setUsage,
		Short:   setShort,
		Long:    str.Dedent(setLong),
		Example: str.Dedent(setExample, 2),
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract command-line arguments.
			includePath := args[0]
			key := args[1]
			value := args[2]

			// Extract command-line flag values.
			fs := cmd.Flags()
			configPath := flags.Config.Get(fs)
			envOptions := getEnvOptions(fs)

			// Obtain the environment service using the configuration path.
			envService, err := f.EnvService(configPath)
			if err != nil {
				return err
			}

			var envTarget string
			if envOptions.Env != nil {
				envTarget = *envOptions.Env
			}

			// Perform the environment set operation.
			result, err := envService.Set(env.SetParams{
				IncludePath: includePath,
				Environment: envTarget,
				Key:         key,
				Value:       value,
			})
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the result of the environment set operation.
			return outputSet(console, result)
		},
	}

	// Bind the command-line flags.
	{
		fs := cmd.Flags()
		flags.Bind(fs, &envFlag)
	}

	return cmd
}

// outputSet confirms the written key and the overlay file it landed in.
func outputSet(console *printer.Printer, result env.SetResult) error {
	return console.LogMessage(fmt.Sprintf(
		"Set %q in:\n%s",
		result.Key,
		str.QuotePath(result.OverlayPath),
	))
}
