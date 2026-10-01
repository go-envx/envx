package cli

import (
	"fmt"
	"io"

	"github.com/go-envx/envx/app/internal/core"
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
	var reveal bool

	cmd := &cobra.Command{
		Use:     getUsage,
		Short:   getShort,
		Long:    str.Dedent(getLong),
		Example: str.Dedent(getExample, 2),
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			key := args[1]

			in := core.GetInput(cmd.Flags())

			envService, err := f.EnvService(in, project)
			if err != nil {
				return err
			}

			result, err := envService.Get(env.GetParams{
				Key:    key,
				Reveal: reveal,
			})
			if err != nil {
				return err
			}

			return outputGet(cmd.OutOrStdout(), result.Value)
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

	flags.Bind(cmd.Flags(), &reveal, &env.Reveal)

	return cmd
}

// outputGet prints the resolved value followed by a newline so the value is
// convenient to pipe.
func outputGet(w io.Writer, value string) error {
	_, err := fmt.Fprintln(w, value)
	return err
}
