package cmd

import (
	"fmt"

	"github.com/go-envx/envx/apps/envx/internal/app"
	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/str"
	"github.com/spf13/cobra"
)

const (
	runUsage = "run <project> <environment> -- <command> [args...]"
	runShort = "Run a command with the merged environment for a project"
	runLong  = `
		Run executes a command with environment variables loaded from the project's
		namespace chain. Variables are merged in order (includes first, project last)
		with later values winning.

		By default, existing OS environment variables take precedence over file values.
		Use --overload to let file values override OS env vars.
	`
	runExample = `
		envx run api-core development -- npm start
		envx run web production -- node server.js
		envx run api-core staging --strict -- ./run.sh
	`
)

// -------------------------------------------------------------------------------------
// newRunCmd creates the "run" subcommand. It receives the shared App instance
// and a pointer to the root-level --config flag value so it can be resolved at
// execution time (after all persistent flags have been parsed).
func newRunCmd(application *app.App, configPath *string) *cobra.Command {
	var flags config.RawFlags

	cmd := &cobra.Command{
		Use:           runUsage,
		Short:         runShort,
		Long:          str.Dedent(runLong),
		Example:       str.Dedent(runExample, 2),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Cobra splits args around "--". ArgsLenAtDash returns the index
			// where "--" appeared, or -1 if it was absent. We require at least
			// two positional args (project + environment) before the dash.
			dashIdx := cmd.ArgsLenAtDash()
			if dashIdx < 0 || dashIdx < 2 {
				return fmt.Errorf("usage: %s", runUsage)
			}

			// Everything before "--" is positional (project, environment);
			// everything after is the child command and its arguments.
			positional := args[:dashIdx]
			childArgs := args[dashIdx:]

			if len(positional) < 2 {
				return fmt.Errorf("usage: %s", runUsage)
			}
			if len(childArgs) == 0 {
				return fmt.Errorf("no command specified after --")
			}

			// Propagate the root-level --config value into the flags struct
			// so the resolver can locate the manifest file.
			flags.ConfigPath = *configPath

			// Delegate to the application layer which handles manifest
			// loading, environment merging, and child process execution.
			return application.Run(
				cmd.Context(),
				flags.ConfigPath,
				positional[0],
				positional[1],
				flags,
				cmd.Flags(),
				app.RunOptions{
					Args:   childArgs,
					Stdout: cmd.OutOrStdout(),
					Stderr: cmd.ErrOrStderr(),
				},
			)
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

	// --namespace-prefix: enabled by default; prefixes each env var with
	// the namespace it was loaded from (e.g. POSTGRES_DB_HOST). Disable
	// with --namespace-prefix=false to export bare key names.
	cmd.Flags().BoolVar(
		&flags.NamespacePrefix,
		"namespace-prefix",
		true,
		"prefix env vars with the file namespace name",
	)

	// Return the constructed command to be added to the root command in NewRootCmd.
	return cmd
}
