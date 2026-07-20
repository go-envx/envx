package create

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/pkg/str"
	"github.com/spf13/cobra"
)

const (
	createUsage = "create <template>"
	createShort = "Scaffold an example envx workspace"
	createLong  = `
		Create scaffolds a ready-to-run envx workspace so you can explore the tool
		without wiring up files by hand. Each template writes a self-contained set
		of envx.yaml, namespace, and overlay files into a target directory.
	`

	quickStartShort = "Scaffold a minimal, single-project workspace"
	quickStartLong  = `
		Scaffold a minimal workspace: one project, a couple of namespaces, and two
		environments — the smallest setup that shows how envx resolves and merges
		environment files.
	`
)

// -------------------------------------------------------------------------------------

// NewCommand builds the "create" command and its per-template subcommands. create
// itself takes no action; each subcommand scaffolds one named template.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   createUsage,
		Short: createShort,
		Long:  str.Dedent(createLong),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newTemplateCmd(quickStart, quickStartShort, quickStartLong),
	)
	return cmd
}

// -------------------------------------------------------------------------------------

// newTemplateCmd builds one "create <template>" subcommand. Each scaffolds its
// named template into --target-dir (defaulting to a directory of the same name),
// refusing to overwrite existing files unless --force is set.
func newTemplateCmd(name, short, long string) *cobra.Command {
	var targetDir string
	var force bool

	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		Long:  str.Dedent(long),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := execute(actionParams{
				Template:  name,
				TargetDir: targetDir,
				Force:     force,
			})
			if err != nil {
				return err
			}
			out := summary(name, targetDir, res.Written)
			if _, err := fmt.Fprint(cmd.OutOrStdout(), out); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&targetDir, "target-dir", name, "directory to scaffold into")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")
	return cmd
}

// -------------------------------------------------------------------------------------

// summary reports the scaffolded files and the first command to try, so the user
// can start exploring immediately.
func summary(name, targetDir string, written []string) string {
	lines := []string{
		fmt.Sprintf("Scaffolded %s into %s/ (%d files):", name, targetDir, len(written)),
	}
	for _, f := range written {
		lines = append(lines, "  "+f)
	}
	lines = append(lines,
		"",
		"Try it:",
		"  cd "+targetDir,
		"  envx get api-service DATABASE_HOST",
	)
	return strings.Join(lines, "\n") + "\n"
}
