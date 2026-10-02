package cli

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/features/scaffold"
	"github.com/go-envx/envx/app/internal/shared/flags"
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

// NewCreateCommand builds the "create" command and its per-template subcommands.
func NewCreateCommand(factory Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   createUsage,
		Short: createShort,
		Long:  str.Dedent(createLong),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newTemplateCommand(factory, templateSpec{
			Name:  scaffold.QuickStartTemplate,
			Short: quickStartShort,
			Long:  quickStartLong,
		}),
	)
	return cmd
}

// newTemplateCommand builds one "create <template>" subcommand.
func newTemplateCommand(f Factory, template templateSpec) *cobra.Command {
	templateDirFlag := template.targetDirFlag()

	cmd := &cobra.Command{
		Use:   template.Name,
		Short: template.Short,
		Long:  str.Dedent(template.Long),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fs := cmd.Flags()
			targetDir := templateDirFlag.Get(fs)

			// Obtain the scaffold service from the factory.
			scaffoldService, err := f.ScaffoldService()
			if err != nil {
				return err
			}

			// Create the workspace template using the scaffold service.
			result, err := scaffoldService.Create(scaffold.CreateParams{
				Template:  template.Name,
				TargetDir: targetDir,
				Force:     forceFlag.Get(fs),
			})
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the result of the scaffold operation.
			return outputCreate(
				console,
				template.Name,
				targetDir,
				result,
			)
		},
	}

	flags.Bind(cmd.Flags(), &templateDirFlag)
	flags.Bind(cmd.Flags(), &forceFlag)

	return cmd
}

// outputCreate writes a human summary of a completed scaffold through the
// shared printer.
func outputCreate(
	console *printer.Printer,
	template string,
	targetDir string,
	result scaffold.CreateResult,
) error {
	lines := []string{
		fmt.Sprintf(
			"Scaffolded %s into %s/ (%d files):",
			template,
			targetDir,
			len(result.Written),
		),
	}
	for _, f := range result.Written {
		lines = append(lines, "  "+f)
	}
	lines = append(lines,
		"",
		"Try it:",
		"  cd "+targetDir,
		"  envx get api-service DATABASE_HOST",
	)
	return console.LogMessage(strings.Join(lines, "\n"))
}
