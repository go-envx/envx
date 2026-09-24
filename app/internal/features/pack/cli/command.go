package cli

import (
	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	usage = "pack --out <dir>"
	short = "Copy an environment-scoped workspace bundle for deployment"
	long  = `
		Pack copies an environment-scoped subset of the workspace into an output
		directory that runs through the ordinary envx pipeline. The bundle contains
		the manifest, each project's include files (the base namespace file plus
		only the selected environments' overlays), and a filtered secrets store
		holding only the values those environments reference.

		It excludes the private-key file — supply it at runtime through
		ENVX_PRIVATE_KEY — every unselected environment's overlays, and (being
		run-only) every public key. Nothing is decrypted: secrets stay encrypted and
		decrypt at runtime exactly as they do for a normal run.

		-e/--env is repeatable and selects one or more environments (default: all
		declared), so a single bundle can serve several environments — the container
		picks one at start with 'envx run api --env production --config
		<dir>/envx.yaml'. --project narrows which projects are copied (default: all).

		The bundle uses a per-project layout: each project gets its own <project>/
		directory holding every namespace file it includes, and the manifest's
		includes are rewritten to <project>/<name> to match. A file two projects both
		include is copied into each directory; the secrets store stays a single
		secrets.yaml at the bundle root, filtered to the union of the selected
		projects' references. Because the store lives at the root, a project directory
		is not runnable on its own — for per-project secret isolation, run pack once
		per project into a separate output.

		An existing, non-empty --out directory is refused so a bundle never merges
		into stale files; pass --force to clear it and pack fresh.
	`
	example = `
		envx pack -e production --out ./dist
		envx pack -e staging -e production --out ./dist
		envx pack -e production -p api-core --out ./dist
	`
)

// NewPackCmd builds the "pack" command, which selects an environment-scoped
// subset of the workspace, copies it into --out, and renders a summary of the
// files written.
func NewPackCmd() *cobra.Command {
	var (
		out          string
		environments []string
		projects     []string
		force        bool
	)

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// resolve the manifest path from the inherited --config flag
			input := core.GetInput(cmd.Flags())

			// copy the selected environments' and projects' files into --out
			result, err := execute(actionParams{
				Environments: environments,
				Projects:     projects,
				OutDir:       out,
				Force:        force,
			}, input)
			if err != nil {
				return err
			}

			// render the summary of what was written
			pr := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})
			return render(pr, result)
		},
	}

	flags.BindString(cmd.Flags(), &out, &Out)
	flags.BindStringSlice(cmd.Flags(), &environments, &Env)
	flags.BindStringSlice(cmd.Flags(), &projects, &Project)
	flags.BindBool(cmd.Flags(), &force, &Force)
	_ = cmd.MarkFlagRequired(Out.Name)

	return cmd
}

// NewCommand is an alias for NewPackCmd.
func NewCommand() *cobra.Command {
	return NewPackCmd()
}
