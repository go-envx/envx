package diff

import (
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/go-envx/envx/apps/envx/internal/schema"
	"github.com/go-envx/envx/apps/envx/internal/str"
	"github.com/spf13/cobra"
)

const (
	usage = "diff <project> <env-a> <env-b>"
	short = "Compare a project's resolved environment across two environments"
	long  = `
		Diff resolves the same project under two environments and reports the
		differences: keys added, removed, or changed between env-a and env-b.

		Values are masked by default; pass --reveal to print them in plaintext.
		Use --output=json for machine-readable output.
	`
	example = `
		envx diff api-core development production
		envx diff api-core development production --reveal
		envx diff api-core development production --output=json
	`
)

// -------------------------------------------------------------------------------------

// NewCommand builds the "diff" command, which parses args into the action's
// params/config, executes the action, and renders the structured diff in the
// specified format.
func NewCommand(configPath *string) *cobra.Command {
	var cfg actionConfig

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ConfigPath = configPath
			cfg.Changed = cmd.Flags()

			// map args to action params
			p := actionParams{
				Project: args[0],
				EnvA:    args[1],
				EnvB:    args[2],
			}

			// execute the action
			res, err := execute(p, &cfg)
			if err != nil {
				return err
			}

			// render the result
			return render(&renderParams{
				Writer: cmd.OutOrStdout(),
				Result: res,
				Format: cfg.Output,
				Reveal: cfg.Reveal,
			})
		},
	}

	flags.BindBool(cmd, &cfg.Settings.Strict, &schema.Strict)
	flags.BindString(cmd, &cfg.Settings.Prefix, &schema.Prefix)
	flags.BindString(cmd, &cfg.Settings.Suffix, &schema.Suffix)
	flags.BindBool(cmd, &cfg.Settings.NamespacePrefix, &schema.NamespacePrefix)
	flags.BindBool(cmd, &cfg.Reveal, &schema.Reveal)
	flags.BindString(cmd, &cfg.Output, &schema.Output)
	return cmd
}
