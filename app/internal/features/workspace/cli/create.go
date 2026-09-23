package cli

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/features/workspace/command"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
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

// CreateHandler defines the contract required by the create CLI command
// to scaffold a workspace.
type CreateHandler interface {
	Execute(cmd command.CreateWorkspaceCommand) (command.CreateWorkspaceResult, error)
}

// CreateFlags holds CLI flags for the create command.
type CreateFlags struct {
	// TargetDir is the directory to scaffold into.
	TargetDir string
	// Force overwrites existing files instead of stopping on conflicts.
	Force bool
}

// NewCreateCmd builds the "create" command and its per-template subcommands. create
// itself takes no action; each subcommand scaffolds one named template.
func NewCreateCmd(handler CreateHandler) *cobra.Command {
	cmd := &cobra.Command{
		Use:   createUsage,
		Short: createShort,
		Long:  str.Dedent(createLong),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newTemplateCmd(
			handler,
			command.QuickStartTemplate,
			quickStartShort,
			quickStartLong,
		),
	)
	return cmd
}

// newTemplateCmd builds one "create <template>" subcommand. Each scaffolds its
// named template into --target-dir (defaulting to a directory of the same name),
// refusing to overwrite existing files unless --force is set.
func newTemplateCmd(
	handler CreateHandler,
	name, short, long string,
) *cobra.Command {
	var flags CreateFlags

	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		Long:  str.Dedent(long),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := handler.Execute(command.CreateWorkspaceCommand{
				Template:  name,
				TargetDir: flags.TargetDir,
				Force:     flags.Force,
			})
			if err != nil {
				return err
			}

			// Render the scaffold summary through the shared printer.
			pr := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})
			return pr.LogMessage(Summary(name, flags.TargetDir, res.Written))
		},
	}
	cmd.Flags().StringVar(
		&flags.TargetDir,
		"target-dir",
		name,
		"directory to scaffold into",
	)
	cmd.Flags().BoolVar(
		&flags.Force,
		"force",
		false,
		"overwrite existing files",
	)
	return cmd
}

// Summary reports the scaffolded files and the first command to try, so the user
// can start exploring immediately.
func Summary(name, targetDir string, written []string) string {
	lines := []string{
		fmt.Sprintf(
			"Scaffolded %s into %s/ (%d files):",
			name,
			targetDir,
			len(written),
		),
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
	return strings.Join(lines, "\n")
}
