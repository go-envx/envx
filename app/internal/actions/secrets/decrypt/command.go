package decrypt

import (
	"io"
	"os"

	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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

			// render the result
			return render(&renderParams{
				Writer:    cmd.OutOrStdout(),
				ErrWriter: cmd.ErrOrStderr(),
				Result:    result,
				Verbose:   verbose,
				Color:     isTerminal(cmd.ErrOrStderr()),
			})
		},
	}

	flags.BindString(cmd.Flags(), &group, &schema.Group)
	flags.BindString(cmd.Flags(), &key, &schema.Key)
	flags.BindBool(cmd.Flags(), &verbose, &schema.Verbose)

	return cmd
}

// isTerminal reports whether the writer is an interactive terminal, so warnings
// are colorized only when a human is watching.
func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}
