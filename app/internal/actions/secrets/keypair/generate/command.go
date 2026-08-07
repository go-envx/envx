package generate

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	usage = "generate [group]"
	short = "Generate a missing secret-group keypair"
	long  = `
		Generate a keypair for GROUP and write its public key to secrets.yaml and
		its private key to the configured private-key file. Existing groups are
		rejected; use keypair rotation when that workflow is available. Pass --stdout
		to generate the configured cipher's keypair without writing files. GROUP may
		be omitted with --stdout to generate an unassigned keypair for manual use.
	`
	example = `
		envx secrets keypair generate production
		envx secrets keypair generate shared
		envx secrets keypair generate production --stdout
		envx secrets keypair generate --stdout
	`
)

// -------------------------------------------------------------------------------------

// NewCommand builds the keypair generation command.
func NewCommand() *cobra.Command {
	var stdout bool

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most 1 arg(s), received %d", len(args))
			}
			if len(args) == 0 && !stdout {
				return errors.New("group is required unless --stdout is set")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// map args to action params
			p := actionParams{}
			if len(args) == 1 {
				p.Group = args[0]
			}

			// load command flags
			flagset := cmd.Flags()
			in := flags.GetInput(flagset)

			// run the action based on the requested output mode
			if stdout {
				return runStdout(cmd, p, in)
			}
			return runPersisted(cmd, p, in)
		},
	}

	flags.BindBool(cmd.Flags(), &stdout, &schema.Stdout)
	return cmd
}

// -------------------------------------------------------------------------------------

// runPersisted executes the file-writing workflow and renders its safe result
// to the command's output stream.
func runPersisted(cmd *cobra.Command, p actionParams, in *config.Input) error {
	output := cmd.OutOrStdout()
	result, err := execute(p, in)
	if err != nil {
		return err
	}
	return render(output, result)
}

// -------------------------------------------------------------------------------------

// runStdout executes the ephemeral workflow and renders its key material to the
// command's output stream. An unassigned keypair also receives guidance on stderr.
func runStdout(cmd *cobra.Command, p actionParams, in *config.Input) error {
	outputWriter := cmd.OutOrStdout()
	errorWriter := cmd.ErrOrStderr()
	result, err := executeStdout(p, in)
	if err != nil {
		return err
	}
	if err := renderStdout(outputWriter, result); err != nil {
		return err
	}
	if p.Group == "" {
		return renderStdoutNotice(errorWriter)
	}
	return nil
}
