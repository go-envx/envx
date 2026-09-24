package cli

import (
	"github.com/go-envx/envx/app/internal/core"
	engine "github.com/go-envx/envx/app/internal/features/pack"
)

// actionParams are the inputs to the pack action.
type actionParams struct {
	// Environments selects the environments to include; empty means all declared.
	Environments []string
	// Projects selects the projects to include; empty means all declared.
	Projects []string
	// OutDir is the destination directory for the bundle.
	OutDir string
	// Force replaces an existing non-empty output directory instead of refusing it.
	Force bool
}

// execute is the imperative shell: resolve the workspace layout, adapt it into
// the engine's Workspace, and copy the environment-scoped file set into the
// output directory. It resolves and decrypts nothing; the engine only selects
// and copies files.
func execute(p actionParams, in *core.Input) (engine.Result, error) {
	// Resolve the manifest into the file-level layout the bundle is built from.
	layout, err := core.ResolveWorkspaceLayout(in)
	if err != nil {
		return engine.Result{}, err
	}

	// Adapt each project's includes into the engine's project shape.
	projects := make([]engine.Project, 0, len(layout.Projects))
	for _, project := range layout.Projects {
		projects = append(projects, engine.Project{
			Name:     project.Name,
			Includes: project.Includes,
		})
	}

	// Copy the selected environments' and projects' files into the output dir.
	return engine.Pack(engine.Workspace{
		ManifestPath: layout.ManifestPath,
		Root:         layout.Root,
		SecretsPath:  layout.SecretsPath,
		Environments: layout.Environments,
		Projects:     projects,
	}, engine.Params{
		Environments: p.Environments,
		Projects:     p.Projects,
		OutDir:       p.OutDir,
		Force:        p.Force,
	})
}
