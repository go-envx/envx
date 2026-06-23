package schema

import "slices"

// -------------------------------------------------------------------------------------

// Manifest is the parsed, validated content of envx.yaml: the declared
// environments, project definitions, and global settings.
type Manifest struct {
	// Global settings apply to all projects and can be overridden by project settings,
	// ENVX_ env vars, and CLI flags.
	Settings Settings `yaml:"settings"`
	// Environments is the list of declared environments. It is used for early validation
	// of the target environment before any file I/O occurs, to catch typos and missing
	// declarations.
	Environments []string `yaml:"environments"`
	// Projects is the map of project name to project definition. Each project declares an
	// ordered list of includes (namespaces) to load and merge for that project, plus
	// optional project-specific settings that override the global settings.
	Projects map[string]Project `yaml:"projects"`
}

// -------------------------------------------------------------------------------------

// Project defines one project's environment configuration.
type Project struct {
	// Includes lists the ordered namespaces to load and merge.
	// Each entry is a relative path of the form "<dir>/<name>"
	// resolving to <dir>/<name>.yaml and <dir>/<name>.<env>.yaml
	Includes []string `yaml:"includes"`
	// Project settings override the global settings.
	Settings Settings `yaml:"settings"`
}

// -------------------------------------------------------------------------------------

// Settings control how envx loads and merges the environment files. Settings can be
// configured globally and overridden at the project level. Settings can also be
// overridden by ENVX_* env vars and CLI flags, which take precedence over the manifest
// values. Booleans are pointers so an explicitly-set false is distinguishable from
// "unset", which the resolution precedence chain relies on when layering project over
// global over CLI input.
type Settings struct {
	// Env is the target environment to load.
	Env             string `yaml:"env"`
	// NamespacePrefix prefixes each key with its namespace name, if true.
	NamespacePrefix *bool  `yaml:"namespace_prefix"`
	// Overload lets file values override existing OS env vars, if true.
	Overload        *bool  `yaml:"overload"`
	// Prefix is prepended to every resolved env-var key.
	Prefix          string `yaml:"prefix"`
	// Strict requires every environment overlay file in the namespace chain to exist.
	Strict          *bool  `yaml:"strict"`
	// Suffix is appended to every resolved env-var key.
	Suffix          string `yaml:"suffix"`
}

// -------------------------------------------------------------------------------------

// DefaultEnvironment is the environment used when none is selected via flag,
// ENVX_ENV, or a manifest env setting: the first declared environment. Every other
// setting defaults to its Go zero value (boolean => false, string => ""). An empty
// string is returned when no environments are declared.
func (m *Manifest) DefaultEnvironment() string {
	if len(m.Environments) == 0 {
		return ""
	}
	return m.Environments[0]
}

// -------------------------------------------------------------------------------------

// HasEnvironment reports whether env is declared in the manifest's environments list.
func (m *Manifest) HasEnvironment(env string) bool {
	return slices.Contains(m.Environments, env)
}

// -------------------------------------------------------------------------------------

// HasInclude reports whether any project declares includePath in its include
// list. It is the pure schema predicate behind the loader's absolute include
// lookup (manifest.Loaded.LookupInclude), which pairs it with the workspace dir.
func (m *Manifest) HasInclude(includePath string) bool {
	for _, p := range m.Projects {
		if slices.Contains(p.Includes, includePath) {
			return true
		}
	}
	return false
}

// -------------------------------------------------------------------------------------

// LookupProject finds a project by name, returning its definition and if one was found.
func (m *Manifest) LookupProject(name string) (Project, bool) {
	p, ok := m.Projects[name]
	return p, ok
}
