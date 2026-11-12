package workspace

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
)

// SchemaDocsURL is the manifest schema reference pointed to when a key is
// rejected, so a user gets the authoritative key list without the error naming
// internal Go types.
const SchemaDocsURL = "https://go-envx.github.io/envx/configuration/schema/"

// LockFilename names the per-directory lock file that coordinates envx writes
// to files in that directory. Adapters keep it Git-ignored.
const LockFilename = ".envx.lock"

// Workspace represents the validated workspace domain entity.
type Workspace struct {
	Path string
	Root string
	// Indent is the block indentation for rewritten YAML, within 2 to 9.
	Indent             int
	Environments       []string
	Projects           map[string]Project
	Settings           Settings
	Secrets            SecretsConfig
	ValidateSeverities map[string]string
}

// Project defines one project's environment configuration within a workspace.
type Project struct {
	// Includes are the namespace prefixes as declared, relative to the workspace root.
	Includes []string
	// IncludePaths are Includes joined onto the workspace root, in the same order.
	IncludePaths []string
	Settings     Settings
}

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

// SecretsConfig configures the workspace-level secrets store. A loaded workspace
// carries absolute paths and a non-empty Cipher.
type SecretsConfig struct {
	SecretsPath string
	KeysPath    string
	Cipher      string
	// Providers declares the enabled external providers.
	Providers ProvidersConfig
}

// ProvidersConfig declares the enabled external secret providers; nil disables
// one.
type ProvidersConfig struct {
	// OnePassword enables op:// references.
	OnePassword *CLIProviderConfig
	// Bitwarden enables bws:// references.
	Bitwarden *BitwardenConfig
	// Passwordstate enables passwordstate:// references.
	Passwordstate *PasswordstateConfig
}

// CLIProviderConfig configures a CLI-backed provider.
type CLIProviderConfig struct {
	// Timeout bounds each CLI invocation; always positive.
	Timeout time.Duration
}

// BitwardenConfig configures the Bitwarden Secrets Manager provider.
type BitwardenConfig struct {
	// Timeout bounds each CLI invocation; always positive.
	Timeout time.Duration
	// Projects maps an environment to the project of its short references.
	Projects map[string]string
}

// PasswordstateConfig configures the Passwordstate REST provider.
type PasswordstateConfig struct {
	// Timeout bounds each HTTP request; always positive.
	Timeout time.Duration
	// URL is the Passwordstate base URL.
	URL string
	// APIKeyEnv names the host variable holding the API key.
	APIKeyEnv string
	// ListIDs maps an environment to the list of its short references.
	ListIDs map[string]int
	// Field is the field read when a reference names none; empty when unset.
	Field string
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
	if _, err := diagnostic.Resolve(w.ValidateSeverities); err != nil {
		return fmt.Errorf("manifest: validate: %w", err)
	}

	providers := w.Secrets.Providers
	if providers.Bitwarden != nil {
		err := checkEnvironmentKeys(w, "bws.project", providers.Bitwarden.Projects)
		if err != nil {
			return err
		}
	}
	if providers.Passwordstate != nil {
		err := checkEnvironmentKeys(
			w, "passwordstate.list-id", providers.Passwordstate.ListIDs,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// checkEnvironmentKeys rejects a per-environment provider setting keyed by an
// undeclared environment.
func checkEnvironmentKeys[V any](
	w *Workspace, setting string, byEnv map[string]V,
) error {
	for _, env := range slices.Sorted(maps.Keys(byEnv)) {
		if !w.HasEnvironment(env) {
			return fmt.Errorf(
				"manifest: secrets.providers.%s: environment %q is not declared",
				setting, env,
			)
		}
	}
	return nil
}
