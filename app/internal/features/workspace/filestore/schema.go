package filestore

import (
	"path/filepath"

	"github.com/go-envx/envx/app/internal/features/workspace"
)

type manifestYAML struct {
	Settings           settingsYAML           `yaml:"settings"`
	Environments       []string               `yaml:"environments"`
	Projects           map[string]projectYAML `yaml:"projects"`
	Secrets            secretsYAML            `yaml:"secrets"`
	ValidateSeverities map[string]string      `yaml:"validate"`
}

func (m manifestYAML) toWorkspace(path string, indent int) *workspace.Workspace {
	projects := make(map[string]workspace.Project, len(m.Projects))
	for name, p := range m.Projects {
		projects[name] = p.toDomain()
	}

	severities := make(map[string]string, len(m.ValidateSeverities))
	for k, v := range m.ValidateSeverities {
		severities[k] = v
	}

	return &workspace.Workspace{
		Path:               path,
		Root:               filepath.Dir(path),
		Indent:             indent,
		Environments:       append([]string(nil), m.Environments...),
		Projects:           projects,
		Settings:           m.Settings.toDomain(),
		Secrets:            m.Secrets.toDomain(),
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

func (p projectYAML) toDomain() workspace.Project {
	return workspace.Project{
		Includes: p.Includes,
		Settings: p.Settings.toDomain(),
	}
}

type secretsYAML struct {
	SecretsPath string `yaml:"path"`
	KeysPath    string `yaml:"keys-path"`
	Cipher      string `yaml:"cipher"`
}

func (s secretsYAML) toDomain() workspace.SecretsConfig {
	return workspace.SecretsConfig{
		SecretsPath: s.SecretsPath,
		KeysPath:    s.KeysPath,
		Cipher:      s.Cipher,
	}
}
