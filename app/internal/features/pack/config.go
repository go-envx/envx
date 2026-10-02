package pack

import (
	"cmp"
	"errors"
	"slices"

	"github.com/go-envx/envx/app/internal/features/workspace"
)

// Config is the file-level view of a resolved workspace pack copies from: the
// manifest and secrets locations, the declared environments, and each project's
// includes. It carries no resolved values because pack never decrypts — it only
// selects and copies files.
type Config struct {
	// ManifestPath is the absolute path of the source manifest.
	ManifestPath string
	// Root is the absolute workspace directory every relative include resolves
	// against.
	Root string
	// SecretsPath is the absolute path of the encrypted secrets store, copied when
	// it exists and skipped for a workspace that has none.
	SecretsPath string
	// Environments is the manifest's declared environment list, the default
	// selection and the set a requested environment is validated against.
	Environments []string
	// Projects is every declared project's includes, sorted by name.
	Projects []Project
}

// Project is one project's contribution to the bundle: its name and the ordered
// include prefixes declared in the manifest (each resolving to a base
// <prefix>.yaml and per-environment <prefix>.<env>.yaml overlays).
type Project struct {
	// Name is the manifest project name.
	Name string
	// Includes lists the project's ordered namespace prefixes, relative to Root.
	Includes []string
}

// LoadConfig derives the pack config from a loaded workspace.
func LoadConfig(ws *workspace.Workspace) (Config, error) {
	if ws == nil {
		return Config{}, errors.New("workspace is required")
	}

	projects := make([]Project, 0, len(ws.Projects))
	for name, p := range ws.Projects {
		projects = append(projects, Project{
			Name:     name,
			Includes: slices.Clone(p.Includes),
		})
	}
	slices.SortFunc(projects, func(a, b Project) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return Config{
		ManifestPath: ws.Path,
		Root:         ws.Root,
		SecretsPath:  ws.Secrets.SecretsPath,
		Environments: slices.Clone(ws.Environments),
		Projects:     projects,
	}, nil
}
