// Package config owns the *mechanism* of configuration: discovery, loading,
// parsing, and precedence resolution, plus the global manifest schema. It does
// not dictate what any single action needs — actions compose their own typed
// config from the shared pieces exported here. The package imports the pure
// flags catalog but no CLI framework, so it stays reusable by a non-cobra
// frontend.
package config

import (
	"path/filepath"
	"slices"
)

// -------------------------------------------------------------------------------------
// Config is the parsed, validated envx.yaml. It holds the declared
// environments, project definitions, and runtime settings.
type Config struct {
	Settings     Settings           `yaml:"settings"`
	Environments []string           `yaml:"environments"`
	Projects     map[string]Project `yaml:"projects"`

	// dir is the directory containing the manifest (the workspace root). It is
	// unexported and used to resolve relative include paths.
	dir string
}

// -------------------------------------------------------------------------------------
// Settings holds runtime configuration declared in the manifest. Booleans are
// pointers so an explicitly-set false is distinguishable from "unset", which
// matters for the precedence chain.
type Settings struct {
	Overload           *bool  `yaml:"overload"`
	Strict             *bool  `yaml:"strict"`
	Prefix             string `yaml:"prefix"`
	Suffix             string `yaml:"suffix"`
	NamespacePrefix    *bool  `yaml:"namespace_prefix"`
	DefaultEnvironment string `yaml:"default_environment"`
}

// -------------------------------------------------------------------------------------
// Project defines one project's environment configuration. Includes lists the
// ordered namespaces to load and merge; each entry is a relative path of the
// form "<dir>/<name>" resolving to <dir>/<name>.yaml and
// <dir>/<name>.<env>.yaml. Project settings override the global settings.
type Project struct {
	Includes []string `yaml:"includes"`
	Settings Settings `yaml:"settings"`
}

// -------------------------------------------------------------------------------------
// ProjectMatch bundles a project's canonical name with its definition so a
// lookup avoids a three-value return.
type ProjectMatch struct {
	Name    string
	Project Project
}

// -------------------------------------------------------------------------------------
// Global is the resolved root context shared by every action: the loaded
// manifest, the path it came from, and the base environment. It is built once
// by cli.PersistentPreRunE so every action observes the same immutable root.
type Global struct {
	Config      *Config
	ConfigPath  string
	Environment string // flag > ENVX_ENV > manifest default > "development"
}

// -------------------------------------------------------------------------------------
// Dir returns the absolute path to the directory containing the manifest. All
// relative include paths are resolved against this directory.
func (c *Config) Dir() string {
	return c.dir
}

// -------------------------------------------------------------------------------------
// LookupProject finds a project by name, returning the match and whether one
// was found.
func (c *Config) LookupProject(name string) (ProjectMatch, bool) {
	if p, ok := c.Projects[name]; ok {
		return ProjectMatch{Name: name, Project: p}, true
	}
	return ProjectMatch{}, false
}

// -------------------------------------------------------------------------------------
// HasEnvironment reports whether env is declared in the manifest's environments
// list. Undeclared environments are rejected early to catch typos before any
// file I/O occurs.
func (c *Config) HasEnvironment(env string) bool {
	return slices.Contains(c.Environments, env)
}

// -------------------------------------------------------------------------------------
// LookupInclude finds an include path across all projects by matching the full
// include string. Returns the resolved absolute directory, the base name, and
// whether a match was found. Used by the set action to locate a target overlay.
func (c *Config) LookupInclude(includePath string) (dir, name string, ok bool) {
	for _, p := range c.Projects {
		if slices.Contains(p.Includes, includePath) {
			absDir := filepath.Join(c.dir, filepath.Dir(includePath))
			return absDir, filepath.Base(includePath), true
		}
	}
	return "", "", false
}
