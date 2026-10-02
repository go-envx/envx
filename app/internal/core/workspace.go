package core

import (
	"sort"

	"github.com/go-envx/envx/app/internal/features/workspace"
)

// WorkspaceLayout is the file-level view of a resolved workspace: the manifest
// location, the shared secrets and private-key paths, the declared environments,
// and each project's includes as declared in the manifest.
type WorkspaceLayout = workspace.Layout

// ProjectIncludes pairs a project name with its includes as declared in the
// manifest (relative prefixes, not yet joined against the workspace directory).
type ProjectIncludes = workspace.ProjectRef

// ResolveWorkspaceLayout resolves the manifest into the file-level view pack
// needs to select and copy a bundle. It loads the manifest once, reads the
// workspace-level secrets and private-key paths, and returns each project's
// includes sorted by name. It constructs no envmerge Manager and opens no
// secrets store, because pack copies files without resolving or decrypting them.
func ResolveWorkspaceLayout(in *Input) (*WorkspaceLayout, error) {
	// Resolve manifest-level config once to read the projects, environments, and
	// the workspace-level secrets and private-key locations.
	base, err := ResolveWorkspace(in)
	if err != nil {
		return nil, err
	}

	// Collect and sort the declared project names for deterministic iteration.
	names := make([]string, 0, len(base.workspace.Projects))
	for name := range base.workspace.Projects {
		names = append(names, name)
	}
	sort.Strings(names)

	// Capture each project's includes verbatim so pack can preserve their layout.
	projects := make([]ProjectIncludes, 0, len(names))
	for _, name := range names {
		projects = append(projects, ProjectIncludes{
			Name:     name,
			Includes: base.workspace.Projects[name].Includes,
		})
	}

	return &WorkspaceLayout{
		ManifestPath: base.path,
		Root:         base.dir,
		SecretsPath:  base.Secrets.SecretsPath,
		KeysPath:     base.Secrets.KeysPath,
		Environments: base.workspace.Environments,
		Projects:     projects,
	}, nil
}
