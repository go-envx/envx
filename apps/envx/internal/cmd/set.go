package cmd

import (
	"github.com/go-envx/envx/apps/envx/internal/app"
	"github.com/go-envx/envx/apps/envx/internal/manifest"
	"github.com/go-envx/envx/apps/envx/internal/str"
	"github.com/spf13/cobra"
)

const (
	setUsage = "set <include-path> <key> <value>"
	setShort = "Set an environment variable in a namespace's overlay file"
	setLong  = `
		Set writes a key-value pair to the environment overlay file for the
		specified include path. The key supports dot notation for nested YAML
		paths (e.g. "credentials.password").

		The include path must match an entry from a project's includes list
		exactly (e.g. "env/postgres", "apps/api-core/env/api-core").

		The target environment is determined by --env flag, ENVX_ENV env var,
		manifest default_environment setting, or defaults to "development".
	`
	setExample = `
		envx set env/postgres password insecure-password
		envx set env/postgres credentials.password s3cret --env=staging
		envx set env/gateway timeout 10 --env=production
	`
)

// -------------------------------------------------------------------------------------
// newSetCmd creates the "set" subcommand. It receives the shared App instance
// and pointers to the root-level --config and --env flag values so they can be
// resolved at execution time (after all persistent flags have been parsed).
func newSetCmd(application *app.App, configPath, envName *string) *cobra.Command {
	var flags manifest.RawFlags

	cmd := &cobra.Command{
		Use:           setUsage,
		Short:         setShort,
		Long:          str.Dedent(setLong),
		Example:       str.Dedent(setExample, 2),
		Args:          cobra.ExactArgs(3),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Propagate root-level persistent flag values into the flags
			// struct so the resolver can use them.
			flags.ConfigPath = *configPath
			flags.Environment = *envName

			// Delegate to the application layer which handles manifest
			// loading, namespace lookup, and YAML file writing.
			return application.Set(&app.SetInput{
				ConfigPath:  flags.ConfigPath,
				IncludePath: args[0],
				Flags:       &flags,
				Changed:     cmd.Flags(),
				Key:         args[1],
				Value:       args[2],
			})
		},
	}

	// Return the constructed command to be added to the root command in NewRootCmd.
	return cmd
}
