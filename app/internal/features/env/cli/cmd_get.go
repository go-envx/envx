package cli

import (
	"fmt"
	"io"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	getUsage = "get <project> <key>"
	getShort = "Get the value of an environment variable for a project"
	getLong  = `
		Get resolves the merged environment for a project and prints the value
		of the specified key. The key is matched case-insensitively (uppercased).

		The target environment is determined by the --env flag, the ENVX_ENV env
		var, a manifest env setting, or defaults to the first environment declared
		in envx.yaml.

		Secret references are masked as "secret://group/key" by default; pass
		--reveal to decrypt and print their plaintext.
	`
	getExample = `
		envx get api-service DATABASE_HOST
		envx get api-service database_host --env=production
		envx get api-service DATABASE_PASSWORD --reveal
	`
)

// NewGetCommand builds the "get" command.
func NewGetCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     getUsage,
		Short:   getShort,
		Long:    str.Dedent(getLong),
		Example: str.Dedent(getExample, 2),
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract command-line arguments.
			project := args[0]
			key := args[1]

			// Extract command-line flag values.
			fs := cmd.Flags()
			configPath := flags.Config.Get(fs)
			reveal := revealFlag.Get(fs)
			envOptions := getEnvOptions(fs)

			// Obtain the environment service using the configuration path.
			envService, err := f.EnvService(configPath)
			if err != nil {
				return err
			}

			// Perform the environment get operation.
			result, err := envService.Get(env.GetParams{
				Project: project,
				Key:     key,
				Reveal:  reveal,
				Options: envOptions,
			})
			if err != nil {
				return err
			}

			// Output the result of the environment get operation.
			return outputGet(cmd.OutOrStdout(), result.Value)
		},
	}

	// Bind the command-line flags.
	{
		fs := cmd.Flags()
		flags.Bind(fs, &revealFlag)
		bindEnvOptionsFlags(fs)
	}

	return cmd
}

// outputGet prints the resolved value followed by a newline so the value is
// convenient to pipe.
func outputGet(w io.Writer, value string) error {
	_, err := fmt.Fprintln(w, value)
	return err
}
