package pack

import (
	"slices"
	"testing"

	"github.com/go-envx/envx/app/internal/features/workspace"
)

// TestLoadConfig verifies pack keeps declared includes, sorts projects by name,
// and carries the manifest, root, and store locations.
func TestLoadConfig(t *testing.T) {
	t.Parallel()

	ws := &workspace.Workspace{
		Path:         "/work/envx.yaml",
		Root:         "/work",
		Environments: []string{"development", "production"},
		Secrets:      workspace.SecretsConfig{SecretsPath: "/work/secrets.yaml"},
		Projects: map[string]workspace.Project{
			"web": {
				Includes:     []string{"env/app"},
				IncludePaths: []string{"/work/env/app"},
			},
			"api": {
				Includes:     []string{"env/app", "env/api"},
				IncludePaths: []string{"/work/env/app", "/work/env/api"},
			},
		},
	}

	config, err := LoadConfig(ws)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.ManifestPath != "/work/envx.yaml" || config.Root != "/work" ||
		config.SecretsPath != "/work/secrets.yaml" {
		t.Errorf("paths = %+v, want manifest, root, and store carried through", config)
	}
	wantEnvs := []string{"development", "production"}
	if !slices.Equal(config.Environments, wantEnvs) {
		t.Errorf("Environments = %v, want %v", config.Environments, wantEnvs)
	}
	want := []Project{
		{Name: "api", Includes: []string{"env/app", "env/api"}},
		{Name: "web", Includes: []string{"env/app"}},
	}
	if len(config.Projects) != len(want) {
		t.Fatalf("Projects = %+v, want %+v", config.Projects, want)
	}
	for i, project := range want {
		got := config.Projects[i]
		if got.Name != project.Name || !slices.Equal(got.Includes, project.Includes) {
			t.Errorf("Projects[%d] = %+v, want %+v", i, got, project)
		}
	}
}

// TestLoadConfigRejectsNilWorkspace verifies a missing workspace is an error.
func TestLoadConfigRejectsNilWorkspace(t *testing.T) {
	t.Parallel()

	if _, err := LoadConfig(nil); err == nil {
		t.Fatal("LoadConfig(nil) succeeded")
	}
}
