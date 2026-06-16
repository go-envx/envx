// Package manifest handles loading, parsing, and validating the envx.yaml
// workspace manifest. The manifest is the central registry that wires projects
// to their shared configuration namespaces.
package manifest

import (
	"path/filepath"
	"slices"
)

// -------------------------------------------------------------------------------------
// Manifest represents a fully parsed and validated envx.yaml file. It holds
// the declared environments, project definitions, and runtime settings.
type Manifest struct {
	Settings     Settings           `yaml:"settings"`
	Environments []string           `yaml:"environments"`
	Projects     map[string]Project `yaml:"projects"`

	// dir is the directory containing the manifest file (the workspace root).
	// Used to resolve relative paths in project definitions.
	dir string
}

// -------------------------------------------------------------------------------------
// Settings holds runtime configuration that can be defined in the manifest,
// overridden by environment variables, and overridden again by CLI flags.
//
// Precedence: CLI flags > env vars > project settings > manifest settings > defaults.
type Settings struct {
	Overload        *bool  `yaml:"overload"`
	Strict          *bool  `yaml:"strict"`
	Prefix          string `yaml:"prefix"`
	Suffix          string `yaml:"suffix"`
	NamespacePrefix *bool  `yaml:"namespace_prefix"`
}

// -------------------------------------------------------------------------------------
// Project defines a single project's environment configuration within the
// manifest. Path points to the directory holding the project's own env files,
// and Includes lists shared namespaces to load before the project's own.
// Settings at the project level override the global manifest settings.
type Project struct {
	Path     string   `yaml:"path"`
	Includes []string `yaml:"includes"`
	Settings Settings `yaml:"settings"`
}

// -------------------------------------------------------------------------------------
// Dir returns the absolute path to the directory containing the manifest file.
// All relative paths in project definitions are resolved against this directory.
func (m *Manifest) Dir() string {
	return m.dir
}

// -------------------------------------------------------------------------------------
// ProjectDir returns the fully resolved path to the project's env directory.
// If the project path is already absolute it is returned as-is; otherwise it
// is joined with the manifest directory.
func (m *Manifest) ProjectDir(p *Project) string {
	if filepath.IsAbs(p.Path) {
		return p.Path
	}
	return filepath.Join(m.dir, p.Path)
}

// -------------------------------------------------------------------------------------
// ProjectMatch holds the result of a project lookup. It bundles the canonical
// project name with the project definition to avoid a 3-value return.
type ProjectMatch struct {
	Name    string
	Project Project
}

// -------------------------------------------------------------------------------------
// LookupProject finds a project by name or by path. It first tries an exact
// name match against the projects map, then falls back to matching the path
// field. Returns the match and whether one was found.
func (m *Manifest) LookupProject(nameOrPath string) (ProjectMatch, bool) {
	// Try direct name match first.
	if p, ok := m.Projects[nameOrPath]; ok {
		return ProjectMatch{Name: nameOrPath, Project: p}, true
	}
	// Try matching by path.
	for name, p := range m.Projects {
		if p.Path == nameOrPath {
			return ProjectMatch{Name: name, Project: p}, true
		}
	}
	return ProjectMatch{}, false
}

// -------------------------------------------------------------------------------------
// HasEnvironment checks whether the given environment name is declared in the
// manifest's environments list. Undeclared environments are rejected early to
// catch typos before any file I/O occurs.
func (m *Manifest) HasEnvironment(env string) bool {
	return slices.Contains(m.Environments, env)
}
