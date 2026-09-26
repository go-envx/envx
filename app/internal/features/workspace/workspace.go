package workspace

import (
	"errors"
	"fmt"
	"slices"

	"github.com/go-envx/envx/app/internal/shared/status"
)

// SchemaDocsURL is the manifest schema reference pointed to when a key is
// rejected, so a user gets the authoritative key list without the error naming
// internal Go types.
const SchemaDocsURL = "https://go-envx.github.io/envx/configuration/schema/"

// Settings holds global and project-level workspace resolution options.
type Settings struct {
	Delimiter        *string
	Env              *string
	Overload         *bool
	Prefix           *string
	ReferencePattern *string
	RequireOverlays  *bool
	Suffix           *string
}

// Project defines one project's environment configuration within a workspace.
type Project struct {
	Includes []string
	Settings Settings
}

// SecretsConfig configures the workspace-level secrets store.
type SecretsConfig struct {
	SecretsPath string
	KeysPath    string
	Cipher      string
}

// Workspace represents the validated workspace domain entity.
type Workspace struct {
	Path               string
	Root               string
	Indent             int
	Environments       []string
	Projects           map[string]Project
	Settings           Settings
	Secrets            SecretsConfig
	ValidateSeverities map[string]string
}

// ProjectRef represents a declared project's includes within a workspace.
type ProjectRef struct {
	// Name is the project name declared in the manifest.
	Name string
	// Includes lists the project's ordered namespace relative paths.
	Includes []string
}

// Layout represents the file-level view of a workspace: manifest path, root
// directory, secrets and keys locations, declared environments, and projects.
type Layout struct {
	// ManifestPath is the absolute path to the manifest file.
	ManifestPath string
	// Root is the absolute root directory of the workspace.
	Root string
	// SecretsPath is the absolute path to the secrets store file.
	SecretsPath string
	// KeysPath is the absolute path to the private-key file.
	KeysPath string
	// Environments is the list of declared environments.
	Environments []string
	// Projects is the list of project includes in sorted order.
	Projects []ProjectRef
}

// DefaultEnvironment returns the first declared environment, or an empty string.
func (w *Workspace) DefaultEnvironment() string {
	if w == nil || len(w.Environments) == 0 {
		return ""
	}
	return w.Environments[0]
}

// HasEnvironment reports whether env is declared in the workspace.
func (w *Workspace) HasEnvironment(env string) bool {
	if w == nil {
		return false
	}
	return slices.Contains(w.Environments, env)
}

// HasInclude reports whether any project declares includePath in its includes.
func (w *Workspace) HasInclude(includePath string) bool {
	if w == nil {
		return false
	}
	for _, project := range w.Projects {
		if slices.Contains(project.Includes, includePath) {
			return true
		}
	}
	return false
}

// LookupProject finds a project by name, returning its definition and
// whether it was found.
func (w *Workspace) LookupProject(name string) (Project, bool) {
	if w == nil {
		return Project{}, false
	}
	p, ok := w.Projects[name]
	return p, ok
}

// Validate enforces the workspace's structural constraints: at least one
// environment and one project must be declared, every project must have at least
// one include, and no include entry may be empty. It reads only the declared
// schema and performs no I/O.
func (w *Workspace) Validate() error {
	if len(w.Environments) == 0 {
		return errors.New("manifest: environments list must not be empty")
	}
	if len(w.Projects) == 0 {
		return errors.New("manifest: at least one project must be defined")
	}

	for name, project := range w.Projects {
		if len(project.Includes) == 0 {
			return fmt.Errorf("manifest: project %q has no includes", name)
		}
		if slices.Contains(project.Includes, "") {
			return fmt.Errorf("manifest: project %q contains an empty include", name)
		}
	}

	// Reject an unknown check or an invalid severity in the validate block so a
	// typo fails at load rather than being silently ignored.
	if _, err := status.Resolve(w.ValidateSeverities); err != nil {
		return fmt.Errorf("manifest: validate: %w", err)
	}

	return nil
}
