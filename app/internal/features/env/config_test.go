package env_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/workspace"
)

func ptr[T any](v T) *T { return &v }

// TestLoadConfigRejectsNilWorkspace verifies a missing workspace is an error.
func TestLoadConfigRejectsNilWorkspace(t *testing.T) {
	t.Parallel()

	if _, err := env.LoadConfig(nil); err == nil {
		t.Fatal("LoadConfig(nil) succeeded")
	}
}

// TestLoadConfigDerivesProjectsAndEnvironments verifies projects carry absolute
// includes and their own settings, and the workspace facts are copied through.
func TestLoadConfigDerivesProjectsAndEnvironments(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "work")
	ws := &workspace.Workspace{
		Root:         root,
		Environments: []string{"development", "production"},
		Projects: map[string]workspace.Project{
			"api": {
				Includes:     []string{"env/app"},
				IncludePaths: []string{filepath.Join(root, "env", "app")},
				Settings:     workspace.Settings{Prefix: ptr("API_"), Env: ptr("production")},
			},
		},
	}

	config, err := env.LoadConfig(ws)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.WorkspaceDir != root {
		t.Errorf("WorkspaceDir = %q, want %q", config.WorkspaceDir, root)
	}
	wantEnvs := []string{"development", "production"}
	if !slices.Equal(config.Environments, wantEnvs) {
		t.Errorf("Environments = %v, want %v", config.Environments, wantEnvs)
	}
	api, ok := config.Projects["api"]
	if !ok {
		t.Fatal("project api missing")
	}
	if api.Name != "api" {
		t.Errorf("Name = %q, want api", api.Name)
	}
	wantIncludes := []string{filepath.Join(root, "env", "app")}
	if !slices.Equal(api.Includes, wantIncludes) {
		t.Errorf("Includes = %v, want %v", api.Includes, wantIncludes)
	}
	if api.Settings.Prefix == nil || *api.Settings.Prefix != "API_" {
		t.Errorf("Settings.Prefix = %v, want API_", api.Settings.Prefix)
	}
}

// TestLoadConfigDefaultEnvironment verifies the global env setting wins over the
// first declared environment.
func TestLoadConfigDefaultEnvironment(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setting *string
		want    string
	}{
		"first declared":     {setting: nil, want: "development"},
		"empty setting":      {setting: ptr(""), want: "development"},
		"global env setting": {setting: ptr("production"), want: "production"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ws := &workspace.Workspace{
				Environments: []string{"development", "production"},
				Settings:     workspace.Settings{Env: tc.setting},
			}
			config, err := env.LoadConfig(ws)
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if config.DefaultEnvironment != tc.want {
				t.Errorf("DefaultEnvironment = %q, want %q", config.DefaultEnvironment, tc.want)
			}
		})
	}
}

// TestLoadConfigGlobalSettings verifies the global settings are carried as plain
// values, with unset knobs left at their zero value.
func TestLoadConfigGlobalSettings(t *testing.T) {
	t.Parallel()

	ws := &workspace.Workspace{
		Settings: workspace.Settings{
			Delimiter:        ptr(";"),
			Prefix:           ptr("APP_"),
			Suffix:           ptr("_X"),
			ReferencePattern: ptr(`\$\{([^}]*)\}`),
			RequireOverlays:  ptr(true),
		},
	}

	config, err := env.LoadConfig(ws)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := env.Settings{
		Delimiter:        ";",
		Prefix:           "APP_",
		Suffix:           "_X",
		ReferencePattern: `\$\{([^}]*)\}`,
		RequireOverlays:  true,
	}
	if config.Settings != want {
		t.Errorf("Settings = %+v, want %+v", config.Settings, want)
	}
}
