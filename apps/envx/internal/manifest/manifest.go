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
	Overload           *bool  `yaml:"overload"`
	Strict             *bool  `yaml:"strict"`
	Prefix             string `yaml:"prefix"`
	Suffix             string `yaml:"suffix"`
	NamespacePrefix    *bool  `yaml:"namespace_prefix"`
	DefaultEnvironment string `yaml:"default_environment"`
}

// -------------------------------------------------------------------------------------
// Project defines a single project's environment configuration within the
// manifest. Includes lists the ordered set of namespaces to load and merge.
// Each include is a relative path in the form "<dir>/<name>" which resolves to
// files <dir>/<name>.yaml and <dir>/<name>.<env>.yaml.
// Settings at the project level override the global manifest settings.
type Project struct {
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
// ProjectMatch holds the result of a project lookup. It bundles the canonical
// project name with the project definition to avoid a 3-value return.
type ProjectMatch struct {
	Name    string
	Project Project
}

// -------------------------------------------------------------------------------------
// LookupProject finds a project by name. Returns the match and whether one was
// found.
func (m *Manifest) LookupProject(name string) (ProjectMatch, bool) {
	if p, ok := m.Projects[name]; ok {
		return ProjectMatch{Name: name, Project: p}, true
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

// -------------------------------------------------------------------------------------
// LookupInclude finds an include path across all projects by matching the full
// include string. This is used by the set command to resolve a user-provided
// include path to its directory and base name. Returns the resolved directory
// and base name, and whether a match was found.
func (m *Manifest) LookupInclude(includePath string) (dir, name string, ok bool) {
	for _, p := range m.Projects {
		if slices.Contains(p.Includes, includePath) {
			absDir := filepath.Join(m.dir, filepath.Dir(includePath))
			return absDir, filepath.Base(includePath), true
		}
	}
	return "", "", false
}
