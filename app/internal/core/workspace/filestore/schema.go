package filestore

import (
	"fmt"
	"maps"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/filex"
)

const (
	// defaultSecretsFilename is the store filename used when the manifest names none.
	defaultSecretsFilename = "secrets.yaml"
	// defaultKeysFilename is the private-key filename placed beside the store by default.
	defaultKeysFilename = "envx.keys"
	// defaultCipher is the algorithm used when the manifest names none.
	defaultCipher = "age"
	// defaultProviderTimeout bounds each external provider call when unset.
	defaultProviderTimeout = 30 * time.Second
	// minIndent and maxIndent bound the YAML block indentation a manifest may imply.
	minIndent = 2
	maxIndent = 9
)

type manifestYAML struct {
	Settings           settingsYAML           `yaml:"settings"`
	Environments       []string               `yaml:"environments"`
	Projects           map[string]projectYAML `yaml:"projects"`
	Secrets            secretsConfigYAML      `yaml:"secrets"`
	ValidateSeverities map[string]string      `yaml:"validate"`
}

func (m manifestYAML) toWorkspace(path string, indent int) *workspace.Workspace {
	root := filepath.Dir(path)

	projects := make(map[string]workspace.Project, len(m.Projects))
	for name, p := range m.Projects {
		projects[name] = p.toDomain(root)
	}

	if indent < minIndent || indent > maxIndent {
		indent = minIndent
	}

	severities := make(map[string]string, len(m.ValidateSeverities))
	maps.Copy(severities, m.ValidateSeverities)

	return &workspace.Workspace{
		Path:               path,
		Root:               root,
		Indent:             indent,
		Environments:       append([]string(nil), m.Environments...),
		Projects:           projects,
		Settings:           m.Settings.toDomain(),
		Secrets:            m.Secrets.toDomain(root),
		ValidateSeverities: severities,
	}
}

type projectYAML struct {
	Includes []string     `yaml:"includes"`
	Settings settingsYAML `yaml:"settings"`
}

func (p projectYAML) toDomain(root string) workspace.Project {
	paths := make([]string, len(p.Includes))
	for i, include := range p.Includes {
		paths[i] = filepath.Join(root, include)
	}
	return workspace.Project{
		Includes:     p.Includes,
		IncludePaths: paths,
		Settings:     p.Settings.toDomain(),
	}
}

type settingsYAML struct {
	Delimiter        *string `yaml:"delimiter"`
	Env              *string `yaml:"env"`
	Overload         *bool   `yaml:"overload"`
	Prefix           *string `yaml:"prefix"`
	ReferencePattern *string `yaml:"reference-pattern"`
	RequireOverlays  *bool   `yaml:"require-overlays"`
	Suffix           *string `yaml:"suffix"`
}

func (s settingsYAML) toDomain() workspace.Settings {
	return workspace.Settings{
		Delimiter:        s.Delimiter,
		Env:              s.Env,
		Overload:         s.Overload,
		Prefix:           s.Prefix,
		ReferencePattern: s.ReferencePattern,
		RequireOverlays:  s.RequireOverlays,
		Suffix:           s.Suffix,
	}
}

type secretsConfigYAML struct {
	SecretsPath string        `yaml:"path"`
	KeysPath    string        `yaml:"keys-path"`
	Cipher      string        `yaml:"cipher"`
	Providers   providersYAML `yaml:"providers"`
}

// toDomain resolves store and key paths against root; the key file defaults
// beside the store.
func (s secretsConfigYAML) toDomain(root string) workspace.SecretsConfig {
	secretsPath := s.SecretsPath
	if secretsPath == "" {
		secretsPath = defaultSecretsFilename
	}
	secretsPath = filex.ResolvePath(root, secretsPath)

	keysPath := filepath.Join(filepath.Dir(secretsPath), defaultKeysFilename)
	if s.KeysPath != "" {
		keysPath = filex.ResolvePath(root, s.KeysPath)
	}

	algorithm := s.Cipher
	if algorithm == "" {
		algorithm = defaultCipher
	}

	return workspace.SecretsConfig{
		SecretsPath: secretsPath,
		KeysPath:    keysPath,
		Cipher:      algorithm,
		Providers:   s.Providers.toDomain(),
	}
}

// providersYAML declares the enabled external providers; nil means disabled.
type providersYAML struct {
	// OnePassword enables op:// references.
	OnePassword *cliProviderYAML `yaml:"op"`
	// Bitwarden enables bws:// references.
	Bitwarden *cliProviderYAML `yaml:"bws"`
	// Passwordstate enables passwordstate:// references.
	Passwordstate *passwordstateYAML `yaml:"passwordstate"`
}

// toDomain lists the declared providers keyed by scheme with default timeouts.
func (p providersYAML) toDomain() map[string]workspace.ProviderConfig {
	providers := make(map[string]workspace.ProviderConfig)
	if p.OnePassword != nil {
		providers["op"] = workspace.ProviderConfig{
			Timeout: p.OnePassword.Timeout.orDefault(),
		}
	}
	if p.Bitwarden != nil {
		providers["bws"] = workspace.ProviderConfig{
			Timeout: p.Bitwarden.Timeout.orDefault(),
		}
	}
	if p.Passwordstate != nil {
		providers["passwordstate"] = workspace.ProviderConfig{
			Timeout:   p.Passwordstate.Timeout.orDefault(),
			URL:       p.Passwordstate.URL,
			APIKeyEnv: p.Passwordstate.APIKeyEnv,
		}
	}
	if len(providers) == 0 {
		return nil
	}
	return providers
}

// cliProviderYAML configures a CLI-backed provider.
type cliProviderYAML struct {
	// Timeout bounds each CLI invocation.
	Timeout durationYAML `yaml:"timeout"`
}

// passwordstateYAML configures the Passwordstate REST provider.
type passwordstateYAML struct {
	// URL is the Passwordstate base URL.
	URL string `yaml:"url"`
	// APIKeyEnv names the host variable holding the API key.
	APIKeyEnv string `yaml:"api-key-env"`
	// Timeout bounds each HTTP request.
	Timeout durationYAML `yaml:"timeout"`
}

// durationYAML decodes a positive Go duration string such as "30s".
type durationYAML time.Duration

// UnmarshalYAML parses a positive Go duration.
func (d *durationYAML) UnmarshalYAML(node *yaml.Node) error {
	var text string
	if err := node.Decode(&text); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(text)
	if err != nil || parsed <= 0 {
		return fmt.Errorf(
			"line %d: timeout %q must be a positive duration such as \"30s\"",
			node.Line, text,
		)
	}
	*d = durationYAML(parsed)
	return nil
}

// orDefault returns the duration, or defaultProviderTimeout when unset.
func (d durationYAML) orDefault() time.Duration {
	if d <= 0 {
		return defaultProviderTimeout
	}
	return time.Duration(d)
}
