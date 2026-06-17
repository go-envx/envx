package cmd

import (
	"fmt"

	"github.com/go-envx/envx/apps/envx/internal/app"
	"github.com/go-envx/envx/apps/envx/internal/manifest"
	"github.com/go-envx/envx/apps/envx/internal/str"
	"github.com/spf13/cobra"
)

const (
	getUsage = "get <project> <key>"
	getShort = "Get the value of an environment variable for a project"
	getLong  = `
		Get resolves the merged environment for a project and prints the value
		of the specified key. The key is matched case-insensitively (uppercased).

		The target environment is determined by --env flag, ENVX_ENV env var,
		manifest default_environment setting, or defaults to "development".
	`
	getExample = `
		envx get api-core POSTGRES_HOST
		envx get api-core postgres_host --env=production
	`
)

// -------------------------------------------------------------------------------------
// newGetCmd creates the "get" subcommand. It receives the shared App instance
// and pointers to the root-level --config and --env flag values so they can be
// resolved at execution time (after all persistent flags have been parsed).
func newGetCmd(application *app.App, configPath, envName *string) *cobra.Command {
	var flags manifest.RawFlags

	cmd := &cobra.Command{
		Use:           getUsage,
		Short:         getShort,
		Long:          str.Dedent(getLong),
		Example:       str.Dedent(getExample, 2),
		Args:          cobra.ExactArgs(2),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Propagate root-level persistent flag values into the flags
			// struct so the resolver can use them.
			flags.ConfigPath = *configPath
			flags.Environment = *envName

			// Delegate to the application layer which handles manifest
			// loading, environment merging, and key lookup.
			val, err := application.Get(
				app.PipelineInput{
					ConfigPath: flags.ConfigPath,
					ProjectRef: args[0],
					Flags:      &flags,
					Changed:    cmd.Flags(),
				},
				args[1],
			)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), val)
			return err
		},
	}

	// --overload: by default OS env vars win over file values; this flag
	// inverts that precedence so file values take priority.
	cmd.Flags().BoolVar(
		&flags.Overload,
		"overload",
		false,
		"let file values override existing OS env vars",
	)

	// --strict: when set, every overlay file referenced in the namespace
	// chain must exist on disk or the command fails with an error.
	cmd.Flags().BoolVar(
		&flags.Strict,
		"strict",
		false,
		"require all environment overlay files to exist",
	)

	// --prefix: static string prepended to every exported env var key
	// (e.g. --prefix=APP_ turns DB_HOST into APP_DB_HOST).
	cmd.Flags().StringVar(
		&flags.Prefix,
		"prefix",
		"",
		"prefix to prepend to all env var keys",
	)

	// --suffix: static string appended to every exported env var key.
	cmd.Flags().StringVar(
		&flags.Suffix,
		"suffix",
		"",
		"suffix to append to all env var keys",
	)

	// --namespace-prefix: disabled by default; when enabled, prefixes each env
	// var with the namespace it was loaded from (e.g. POSTGRES_HOST). Enable
	// with --namespace-prefix=true to add file-level namespace prefixes.
	cmd.Flags().BoolVar(
		&flags.NamespacePrefix,
		"namespace-prefix",
		false,
		"prefix env vars with the file namespace name",
	)

	// Return the constructed command to be added to the root command in NewRootCmd.
	return cmd
}
