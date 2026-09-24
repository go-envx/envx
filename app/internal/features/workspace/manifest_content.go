package workspace

import (
	"errors"
	"fmt"
	"slices"

	"github.com/go-envx/envx/app/internal/shared/status"
)

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
	// Secrets configures the workspace-level secrets store.
	Secrets SecretsConfig `yaml:"secrets"`
	// ValidateSeverities configures the validate command's per-check severities,
	// keyed by the lowercased check code (e.g. "secret_is_not_referenced") with an
	// off/warn/error value. It is read only from envx.yaml — no flag or env var
	// overrides it — so severity policy stays a committed property of the workspace.
	ValidateSeverities map[string]string `yaml:"validate"`
}

// SecretsConfig configures the workspace-level secrets store. Its zero value
// discovers secrets.yaml beside the manifest.
type SecretsConfig struct {
	// SecretsPath overrides the secrets file location. A relative path is joined
	// against the manifest directory.
	SecretsPath string `yaml:"path"`
	// KeysPath overrides the private-key file location. A relative path is joined
	// against the manifest directory.
	KeysPath string `yaml:"keys-path"`
	// Cipher selects the encryption algorithm used for new keypairs and values.
	Cipher string `yaml:"cipher"`
}

// Project defines one project's environment configuration.
type Project struct {
	// Includes lists the ordered namespaces to load and merge.
	// Each entry is a relative path of the form "<dir>/<name>"
	// resolving to <dir>/<name>.yaml and <dir>/<name>.<env>.yaml
	Includes []string `yaml:"includes"`
	// Project settings override the global settings.
	Settings Settings `yaml:"settings"`
}

// Settings control how envx loads and merges the environment files. Settings can be
// configured globally and overridden at the project level. Settings can also be
// overridden by ENVX_* env vars and CLI flags, which take precedence over the manifest
// values. Optional scalar values are pointers so the parsed manifest preserves whether
// a setting was omitted; config resolution currently treats omitted and empty strings
// the same while retaining the distinction for future precedence changes.
type Settings struct {
	// Delimiter joins a list-valued leaf into a single env var.
	Delimiter *string `yaml:"delimiter"`
	// Env is the target environment to load.
	Env *string `yaml:"env"`
	// Overload lets file values override existing OS env vars, if true.
	Overload *bool `yaml:"overload"`
	// Prefix is prepended to every resolved env-var key.
	Prefix *string `yaml:"prefix"`
	// ReferencePattern overrides the {{VAR}} reference syntax with a regex.
	ReferencePattern *string `yaml:"reference-pattern"`
	// RequireOverlays requires every environment overlay file in the namespace to exist.
	RequireOverlays *bool `yaml:"require-overlays"`
	// Suffix is appended to every resolved env-var key.
	Suffix *string `yaml:"suffix"`
}

// DefaultEnvironment is the environment used when none is selected via flag,
// ENVX_ENV, or a manifest env setting: the first declared environment. An empty
// string is returned when no environments are declared.
func (m *Manifest) DefaultEnvironment() string {
	if len(m.Environments) == 0 {
		return ""
	}
	return m.Environments[0]
}

// HasEnvironment reports whether env is declared in the manifest's environments list.
func (m *Manifest) HasEnvironment(env string) bool {
	return slices.Contains(m.Environments, env)
}

// HasInclude reports whether any project declares includePath in its include
// list. It is the pure schema predicate config uses to validate a set target
// before joining the include against the workspace directory.
func (m *Manifest) HasInclude(includePath string) bool {
	for _, project := range m.Projects {
		if slices.Contains(project.Includes, includePath) {
			return true
		}
	}
	return false
}

// LookupProject finds a project by name, returning its definition and if one was found.
func (m *Manifest) LookupProject(name string) (Project, bool) {
	p, ok := m.Projects[name]
	return p, ok
}

// Validate enforces the manifest's structural constraints: at least one
// environment and one project must be declared, every project must have at least
// one include, and no include entry may be empty. It reads only the declared
// schema and performs no I/O.
func (m *Manifest) Validate() error {
	if len(m.Environments) == 0 {
		return errors.New("manifest: environments list must not be empty")
	}
	if len(m.Projects) == 0 {
		return errors.New("manifest: at least one project must be defined")
	}

	for name, project := range m.Projects {
		if len(project.Includes) == 0 {
			return fmt.Errorf("manifest: project %q has no includes", name)
		}
		if slices.Contains(project.Includes, "") {
			return fmt.Errorf("manifest: project %q contains an empty include", name)
		}
	}

	// Reject an unknown check or an invalid severity in the validate block so a
	// typo fails at load rather than being silently ignored.
	if _, err := status.Resolve(m.ValidateSeverities); err != nil {
		return fmt.Errorf("manifest: validate: %w", err)
	}

	return nil
}
