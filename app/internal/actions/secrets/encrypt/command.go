package encrypt

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "encrypt"
	short = "Encrypt plaintext values in the store"
	long  = `
		Encrypt re-encrypts plaintext values in the workspace store in place using
		each group's public key. Values that are already encrypted are left
		untouched, so the command is safe to run repeatedly. Use --group and --key
		to narrow the operation; by default every plaintext value is encrypted. A
		--group or --key that matches no stored value is an error.
	`
	example = `
		envx secrets encrypt
		envx secrets encrypt --group production
		envx secrets encrypt --group shared --key service_token
	`
)

// NewCommand builds the command that encrypts plaintext store values in place.
func NewCommand() *cobra.Command {
	var (
		group, key string
		verbose    bool
	)

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// map flags to action params
			p := actionParams{Group: group, Key: key}

			// load command flags
			in := flags.GetInput(cmd.Flags())

			// execute the action
			result, err := execute(p, in)
			if err != nil {
				return err
			}

			// render the result
			return render(&renderParams{
				Writer:  cmd.OutOrStdout(),
				Result:  result,
				Verbose: verbose,
			})
		},
	}

	flags.BindString(cmd.Flags(), &group, &schema.Group)
	flags.BindString(cmd.Flags(), &key, &schema.Key)
	flags.BindBool(cmd.Flags(), &verbose, &schema.Verbose)

	return cmd
}
