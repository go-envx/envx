package cli

import (
	"github.com/go-envx/envx/app/internal/core"
	engine "github.com/go-envx/envx/app/internal/features/validate"
)

// actionParams are the inputs to the validate action.
type actionParams struct {
	// Strict fails the run on warnings as well as errors.
	Strict bool
	// Selected lists the checks to run, keyed by canonical status code. An empty
	// map runs every check; a non-empty map runs only the selected checks and skips
	// the cost of the rest.
	Selected map[string]bool
}

// execute is the imperative shell: resolve every project in the workspace, wire
// the shared secrets manager, and run the workspace-wide diagnosis. It never
// materializes plaintext; the engine diagnoses references through the masked
// dry-run path. The returned report is sorted for stable output.
func execute(p actionParams, in *core.Input) (engine.Report, error) {
	// Resolve every declared project into a build-ready configuration.
	workspace, err := core.ResolveWorkspaceProjects(in)
	if err != nil {
		return engine.Report{}, err
	}

	// Compose the shared secrets manager for the store-level findings.
	manager, err := core.NewSecretsManager(workspace.Secrets, workspace.Cipher)
	if err != nil {
		return engine.Report{}, err
	}

	// Adapt each resolved project into the engine's project manager.
	projects := make([]engine.ProjectManager, 0, len(workspace.Projects))
	for _, project := range workspace.Projects {
		projects = append(projects, engine.ProjectManager{
			Name:    project.Name,
			Manager: project.Result.Envmerge,
		})
	}

	// Run the workspace-wide diagnosis and grade it under the strict policy and the
	// per-check severity overrides resolved from the manifest.
	report, err := engine.Validate(engine.Workspace{
		Projects:     projects,
		Environments: workspace.Environments,
		Secrets:      manager,
	}, engine.Params{Strict: p.Strict, Severity: workspace.Severity, Selected: p.Selected})
	if err != nil {
		return engine.Report{}, err
	}

	report.Sort()
	return report, nil
}
