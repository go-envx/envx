package filestore

import (
	"path/filepath"

	"github.com/go-envx/envx/app/internal/features/workspace"
	"github.com/go-envx/envx/app/internal/utils/filex"
)

const (
	// defaultSecretsFilename is the store filename used when the manifest names none.
	defaultSecretsFilename = "secrets.yaml"
	// defaultKeysFilename is the private-key filename placed beside the store by default.
	defaultKeysFilename = "envx.keys"
	// defaultCipher is the algorithm used when the manifest names none.
	defaultCipher = "age"
	// minIndent and maxIndent bound the YAML block indentation a manifest may imply.
	minIndent = 2
	maxIndent = 9
)

type manifestYAML struct {
	Settings           settingsYAML           `yaml:"settings"`
	Environments       []string               `yaml:"environments"`
	Projects           map[string]projectYAML `yaml:"projects"`
	Secrets            secretsYAML            `yaml:"secrets"`
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
	for k, v := range m.ValidateSeverities {
		severities[k] = v
	}

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

type secretsYAML struct {
	SecretsPath string `yaml:"path"`
	KeysPath    string `yaml:"keys-path"`
	Cipher      string `yaml:"cipher"`
}

// toDomain resolves store and key paths against root; the key file defaults
// beside the store.
func (s secretsYAML) toDomain(root string) workspace.SecretsConfig {
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
	}
}
