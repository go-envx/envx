package cli

import (
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

// scaffoldService defines the contract required by the create CLI command
// to scaffold a workspace.
type scaffoldService interface {
	Create(params scaffold.CreateParams) (scaffold.CreateResult, error)
}

// templateSpec describes the metadata for a single template subcommand.
type templateSpec struct {
	Name  string
	Short string
	Long  string
}

// NewCreateCmd builds the "create" command and its per-template subcommands.
func NewCreateCmd(svc scaffoldService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   createUsage,
		Short: createShort,
		Long:  str.Dedent(createLong),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newTemplateCmd(svc, templateSpec{
			Name:  scaffold.QuickStartTemplate,
			Short: quickStartShort,
			Long:  quickStartLong,
		}),
	)
	return cmd
}

// newTemplateCmd builds one "create <template>" subcommand.
func newTemplateCmd(svc scaffoldService, spec templateSpec) *cobra.Command {
	var (
		targetDir string
		force     bool
	)

	cmd := &cobra.Command{
		Use:   spec.Name,
		Short: spec.Short,
		Long:  str.Dedent(spec.Long),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := svc.Create(scaffold.CreateParams{
				Template:  spec.Name,
				TargetDir: targetDir,
				Force:     force,
			})
			if err != nil {
				return err
			}

			// Render the scaffold summary through the shared printer.
			pr := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})
			return render(pr, summaryParams{
				Template:  spec.Name,
				TargetDir: targetDir,
				Written:   res.Written,
			})
		},
	}

	flags.Bind(cmd.Flags(), &targetDir, targetDirFor(spec.Name))
	flags.Bind(cmd.Flags(), &force, &forceFlag)

	return cmd
}
