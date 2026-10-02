package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	packUsage = "pack --out <dir>"
	packShort = "Copy an environment-scoped workspace bundle for deployment"
	packLong  = `
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
	packExample = `
		envx pack -e production --out ./dist
		envx pack -e staging -e production --out ./dist
		envx pack -e production -p api-core --out ./dist
	`
)

// NewPackCommand builds the "pack" command.
func NewPackCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     packUsage,
		Short:   packShort,
		Long:    str.Dedent(packLong),
		Example: str.Dedent(packExample, 2),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Extract command-line flag values.
			fs := cmd.Flags()
			configPath := flags.Config.Get(fs)
			environments := envFlag.Get(fs)
			projects := projectFlag.Get(fs)
			out := outFlag.Get(fs)
			force := forceFlag.Get(fs)

			// Obtain the pack service using the configuration path.
			packService, err := f.PackService(configPath)
			if err != nil {
				return err
			}

			// Copy the selected environments' and projects' files into the output
			// directory.
			result, err := packService.Pack(pack.PackParams{
				Environments: environments,
				Projects:     projects,
				OutDir:       out,
				Force:        force,
			})
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the summary of what was written.
			return outputPack(console, result)
		},
	}

	// Bind the command-line flags.
	{
		fs := cmd.Flags()
		flags.Bind(fs, &outFlag)
		flags.Bind(fs, &envFlag)
		flags.Bind(fs, &projectFlag)
		flags.Bind(fs, &forceFlag)
		_ = cmd.MarkFlagRequired(outFlag.Name)
	}

	return cmd
}

// outputPack reports the destination, the included environments, and the copied
// files, followed by the command that runs a packed project.
func outputPack(console *printer.Printer, result pack.PackResult) error {
	lines := []string{
		fmt.Sprintf(
			"Packed %d files into %s for environments [%s]:",
			len(result.Files),
			result.OutDir,
			strings.Join(result.Environments, ", "),
		),
	}
	for _, f := range result.Files {
		lines = append(lines, "  "+f)
	}
	lines = append(lines,
		"",
		"Run one of the packed projects:",
		fmt.Sprintf(
			"  envx run %s --env %s --config %s -- <command>",
			firstOr(result.Projects, "<project>"),
			firstOr(result.Environments, "<env>"),
			filepath.Join(result.OutDir, result.ManifestFile),
		),
	)
	return console.LogMessage(strings.Join(lines, "\n"))
}

// firstOr returns the first element of values, or fallback when values is empty,
// so the run hint names a concrete project and environment when one exists and a
// placeholder otherwise.
func firstOr(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return values[0]
}
