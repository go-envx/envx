package config

import (
	"sort"

	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/features/workspace"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/status"
)

// WorkspaceProject pairs a project name with its resolved, build-ready Result so
// a workspace-wide command can iterate every project through the same envmerge
// Manager the single-project actions use.
type WorkspaceProject struct {
	// Name is the manifest project name.
	Name string
	// Result is the project's build-ready configuration, including its constructed
	// envmerge Manager.
	Result *Result
}

// WorkspaceProjects is the fully resolved workspace: one built Result per
// declared project, the declared environments, and the shared secrets and cipher
// parameters. Commands that enumerate every project and environment (validate,
// and later audit) iterate it instead of resolving one project at a time.
type WorkspaceProjects struct {
	// Projects is every declared project, resolved and sorted by name for
	// deterministic iteration and output.
	Projects []WorkspaceProject
	// Environments is the manifest's declared environment list.
	Environments []string
	// Secrets locates the workspace secrets store and private-key file, shared
	// across projects because secrets are workspace-level.
	Secrets secrets.Params
	// Cipher holds the configured cipher construction parameters.
	Cipher cipher.Params
	// Severity is the resolved per-check severity override from the manifest's
	// validate block, keyed by canonical status code. Nil means every check keeps
	// its default severity.
	Severity map[string]status.Severity
}

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
	names := make([]string, 0, len(base.manifest.Projects))
	for name := range base.manifest.Projects {
		names = append(names, name)
	}
	sort.Strings(names)

	// Capture each project's includes verbatim so pack can preserve their layout.
	projects := make([]ProjectIncludes, 0, len(names))
	for _, name := range names {
		projects = append(projects, ProjectIncludes{
			Name:     name,
			Includes: base.manifest.Projects[name].Includes,
		})
	}

	return &WorkspaceLayout{
		ManifestPath: base.path,
		Root:         base.dir,
		SecretsPath:  base.Secrets.SecretsPath,
		KeysPath:     base.Secrets.KeysPath,
		Environments: base.manifest.Environments,
		Projects:     projects,
	}, nil
}

// ResolveWorkspaceProjects resolves every project declared in the manifest into a
// build-ready Result, alongside the declared environments and the shared secrets
// store parameters. It is the workspace-wide counterpart to ResolveProject: it
// loads the manifest once to enumerate projects, then resolves each project
// through the same path so every Result carries a fully constructed envmerge
// Manager. Projects are returned in sorted name order so iteration and output
// stay deterministic.
func ResolveWorkspaceProjects(in *Input) (*WorkspaceProjects, error) {
	// Resolve manifest-level config once to enumerate projects and environments
	// and to read the workspace-level secrets and cipher parameters.
	base, err := ResolveWorkspace(in)
	if err != nil {
		return nil, err
	}

	// Collect and sort the declared project names for deterministic iteration.
	names := make([]string, 0, len(base.manifest.Projects))
	for name := range base.manifest.Projects {
		names = append(names, name)
	}
	sort.Strings(names)

	// Resolve each project into a build-ready Result with its own envmerge Manager.
	projects := make([]WorkspaceProject, 0, len(names))
	for _, name := range names {
		res, err := ResolveProject(in, name)
		if err != nil {
			return nil, err
		}
		projects = append(projects, WorkspaceProject{Name: name, Result: res})
	}

	// Resolve the validate severity overrides. The manifest already validated the
	// block at load, so this only re-keys it by canonical code.
	severity, err := status.Resolve(base.manifest.ValidateSeverities)
	if err != nil {
		return nil, err
	}

	return &WorkspaceProjects{
		Projects:     projects,
		Environments: base.manifest.Environments,
		Secrets:      base.Secrets,
		Cipher:       base.Cipher,
		Severity:     severity,
	}, nil
}
