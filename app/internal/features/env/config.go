package env

import (
	"errors"

	"github.com/go-envx/envx/app/internal/features/workspace"
)

// Config is everything the workspace manifest configures for environment resolution.
type Config struct {
	// WorkspaceDir is the workspace root directory.
	WorkspaceDir string
	// Projects maps declared project names to their definitions.
	Projects map[string]ProjectConfig
	// Environments lists the declared environments, used to validate the target.
	Environments []string
	// DefaultEnvironment is the precedence-resolved default an operation uses when
	// it is given no explicit environment.
	DefaultEnvironment string
	// Settings holds the fully-resolved env-resolution knobs the merge reads.
	Settings Settings
}

// LoadConfig derives the environment config from a loaded workspace.
func LoadConfig(ws *workspace.Workspace) (Config, error) {
	if ws == nil {
		return Config{}, errors.New("workspace is required")
	}

	projects := make(map[string]ProjectConfig, len(ws.Projects))
	for name, p := range ws.Projects {
		projects[name] = ProjectConfig{
			Name:     name,
			Includes: append([]string(nil), p.IncludePaths...),
			Settings: Options{
				Delimiter:        p.Settings.Delimiter,
				Env:              p.Settings.Env,
				Overload:         p.Settings.Overload,
				Prefix:           p.Settings.Prefix,
				ReferencePattern: p.Settings.ReferencePattern,
				RequireOverlays:  p.Settings.RequireOverlays,
				Suffix:           p.Settings.Suffix,
			},
		}
	}

	defaultEnv := ws.DefaultEnvironment()
	if ws.Settings.Env != nil && *ws.Settings.Env != "" {
		defaultEnv = *ws.Settings.Env
	}

	return Config{
		WorkspaceDir:       ws.Root,
		Projects:           projects,
		Environments:       append([]string(nil), ws.Environments...),
		DefaultEnvironment: defaultEnv,
		Settings: Settings{
			Delimiter:        PrecedenceString(ws.Settings.Delimiter),
			Prefix:           PrecedenceString(ws.Settings.Prefix),
			Suffix:           PrecedenceString(ws.Settings.Suffix),
			ReferencePattern: PrecedenceString(ws.Settings.ReferencePattern),
			RequireOverlays:  PrecedenceBool(ws.Settings.RequireOverlays),
			Overload:         PrecedenceBool(ws.Settings.Overload),
		},
	}, nil
}
