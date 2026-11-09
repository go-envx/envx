package pack

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/go-envx/envx/app/internal/core/workspace"
)

// Bundles use envx's default filenames whatever the source called its files, so
// every bundle runs with the same `envx run --config <dir>/envx.yaml`.
const (
	bundleManifestName = "envx.yaml"
	bundleSecretsName  = "secrets.yaml"
)

// Service selects environment-scoped workspace files and publishes them as a
// deployable bundle.
type Service struct {
	// manifestPath is the absolute source manifest path.
	manifestPath string
	// secretsPath is the absolute source secrets store path, if configured.
	secretsPath string
	// environments lists the declared environments in order.
	environments []string
	// projects lists the declared projects sorted by name.
	projects []project
	// protected are workspace files and include directories, whether selected
	// or not, that no destination may be or contain.
	protected []string
	// deps holds the adapters.
	deps Dependencies
}

// project is one declared project's name and includes.
type project struct {
	// name is the manifest project name.
	name string
	// includes are the project's includes in declaration order.
	includes []include
}

// include pairs a declared include with its absolute path prefix.
type include struct {
	// rel is the include as written in the manifest.
	rel string
	// path is the include joined onto the workspace root.
	path string
}

// New privately copies the workspace configuration without performing I/O.
func New(ws workspace.Workspace, deps Dependencies) (*Service, error) {
	if deps.Secrets == nil {
		return nil, errors.New("secrets reader is required")
	}
	if deps.ExportSecrets == nil {
		return nil, errors.New("secrets exporter is required")
	}
	protected := []string{ws.Path}
	for _, path := range []string{ws.Secrets.SecretsPath, ws.Secrets.KeysPath} {
		if path != "" {
			protected = append(protected, path)
		}
	}
	projects := make([]project, 0, len(ws.Projects))
	for name, p := range ws.Projects {
		if len(p.IncludePaths) != len(p.Includes) {
			return nil, fmt.Errorf("project %q has unresolved include paths", name)
		}
		includes := make([]include, len(p.Includes))
		for i, rel := range p.Includes {
			includes[i] = include{rel: rel, path: p.IncludePaths[i]}
			protected = append(protected, filepath.Dir(p.IncludePaths[i]))
		}
		projects = append(projects, project{name: name, includes: includes})
	}
	slices.SortFunc(projects, func(a, b project) int {
		return cmp.Compare(a.name, b.name)
	})
	return &Service{
		manifestPath: ws.Path,
		secretsPath:  ws.Secrets.SecretsPath,
		environments: slices.Clone(ws.Environments),
		projects:     projects,
		protected:    protected,
		deps:         deps,
	}, nil
}

// exists reports whether path names an existing regular file.
func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
