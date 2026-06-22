package manifest

import (
	"path/filepath"
	"slices"

	"github.com/go-envx/envx/apps/envx/internal/settings"
)

// -------------------------------------------------------------------------------------
// Manifest is the parsed, validated envx.yaml. It holds the declared
// environments, project definitions, and runtime settings.
type Manifest struct {
	Settings     settings.File      `yaml:"settings"`
	Environments []string           `yaml:"environments"`
	Projects     map[string]Project `yaml:"projects"`

	// dir is the directory containing the manifest (the workspace root). It is
	// unexported and used to resolve relative include paths.
	dir string
}

// -------------------------------------------------------------------------------------
// Project defines one project's environment configuration. Includes lists the
// ordered namespaces to load and merge; each entry is a relative path of the
// form "<dir>/<name>" resolving to <dir>/<name>.yaml and
// <dir>/<name>.<env>.yaml. Project settings override the global settings.
type Project struct {
	Includes []string      `yaml:"includes"`
	Settings settings.File `yaml:"settings"`
}

// -------------------------------------------------------------------------------------
// ProjectMatch bundles a project's canonical name with its definition so a
// lookup avoids a three-value return.
type ProjectMatch struct {
	Name    string
	Project Project
}

// -------------------------------------------------------------------------------------
// Dir returns the absolute path to the directory containing the manifest. All
// relative include paths are resolved against this directory.
func (m *Manifest) Dir() string {
	return m.dir
}

// -------------------------------------------------------------------------------------
// LookupProject finds a project by name, returning the match and whether one
// was found.
func (m *Manifest) LookupProject(name string) (ProjectMatch, bool) {
	if p, ok := m.Projects[name]; ok {
		return ProjectMatch{Name: name, Project: p}, true
	}
	return ProjectMatch{}, false
}

// -------------------------------------------------------------------------------------
// HasEnvironment reports whether env is declared in the manifest's environments
// list. Undeclared environments are rejected early to catch typos before any
// file I/O occurs.
func (m *Manifest) HasEnvironment(env string) bool {
	return slices.Contains(m.Environments, env)
}

// -------------------------------------------------------------------------------------
// LookupInclude finds an include path across all projects by matching the full
// include string. Returns the resolved absolute directory, the base name, and
// whether a match was found. Used by the set action to locate a target overlay.
func (m *Manifest) LookupInclude(includePath string) (dir, name string, ok bool) {
	for _, p := range m.Projects {
		if slices.Contains(p.Includes, includePath) {
			absDir := filepath.Join(m.dir, filepath.Dir(includePath))
			return absDir, filepath.Base(includePath), true
		}
	}
	return "", "", false
}
