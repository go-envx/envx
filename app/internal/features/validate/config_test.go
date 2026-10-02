package validate

import (
	"slices"
	"testing"

	"github.com/go-envx/envx/app/internal/features/workspace"
	"github.com/go-envx/envx/app/internal/shared/status"
)

// TestLoadConfig verifies projects are sorted by name and severities are re-keyed
// through status.Resolve.
func TestLoadConfig(t *testing.T) {
	t.Parallel()

	ws := &workspace.Workspace{
		Environments: []string{"development", "production"},
		Projects: map[string]workspace.Project{
			"web": {},
			"api": {},
		},
		ValidateSeverities: map[string]string{"secret_is_not_referenced": "off"},
	}

	config, err := LoadConfig(ws)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if want := []string{"api", "web"}; !slices.Equal(config.Projects, want) {
		t.Errorf("Projects = %v, want %v", config.Projects, want)
	}
	wantEnvs := []string{"development", "production"}
	if !slices.Equal(config.Environments, wantEnvs) {
		t.Errorf("Environments = %v, want %v", config.Environments, wantEnvs)
	}
	if got := config.Severity[status.SecretIsNotReferenced]; got != status.Off {
		t.Errorf("Severity[%s] = %v, want off", status.SecretIsNotReferenced, got)
	}
}

// TestLoadConfigRejectsInvalidSeverity verifies an unknown check fails the load.
func TestLoadConfigRejectsInvalidSeverity(t *testing.T) {
	t.Parallel()

	ws := &workspace.Workspace{
		ValidateSeverities: map[string]string{"no_such_check": "off"},
	}
	if _, err := LoadConfig(ws); err == nil {
		t.Fatal("LoadConfig accepted an unknown check")
	}
}

// TestLoadConfigRejectsNilWorkspace verifies a missing workspace is an error.
func TestLoadConfigRejectsNilWorkspace(t *testing.T) {
	t.Parallel()

	if _, err := LoadConfig(nil); err == nil {
		t.Fatal("LoadConfig(nil) succeeded")
	}
}
