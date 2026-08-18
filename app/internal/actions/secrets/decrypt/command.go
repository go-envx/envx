package decrypt

import (
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "decrypt"
	short = "Decrypt stored values into plaintext"
	long  = `
		Decrypt rewrites encrypted values in the workspace store as plaintext in
		place, using each group's private key. Values that are already plaintext are
		left untouched, so the command is safe to run repeatedly. Use --group and
		--key to narrow the operation; by default every encrypted value is decrypted.

		A group whose private key is unavailable is skipped with a warning, so the
		groups you can decrypt still succeed. A --group or --key that matches no
		stored value is an error.

		Decryption writes plaintext secrets to disk. Re-encrypt with
		'envx secrets encrypt' before committing the store.
	`
	example = `
		envx secrets decrypt
		envx secrets decrypt --group production
		envx secrets decrypt --group shared --key service_token
	`
)

// NewCommand builds the command that decrypts stored values in place.
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

			// render the result through the shared printer
			pr := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})
			return render(&renderParams{
				Printer: pr,
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
