package config

import (
	"testing"

	"github.com/go-envx/envx/app/internal/fixtures"
)

// TestResolveWorkspaceProjects verifies every declared project is resolved into a
// build-ready result with an envmerge Manager, returned in sorted name order,
// alongside the declared environments and the shared secrets parameters.
func TestResolveWorkspaceProjects(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	workspace, err := ResolveWorkspaceProjects(&Input{ConfigPath: &path})
	if err != nil {
		t.Fatalf("ResolveWorkspaceProjects(): %v", err)
	}

	names := make([]string, 0, len(workspace.Projects))
	for _, p := range workspace.Projects {
		if p.Result == nil || p.Result.Envmerge == nil {
			t.Errorf("project %q has no built envmerge manager", p.Name)
		}
		names = append(names, p.Name)
	}
	// The basic fixture declares api-core and web; names come back sorted.
	if len(names) != 2 || names[0] != "api-core" || names[1] != "web" {
		t.Errorf("projects = %v, want [api-core web]", names)
	}

	wantEnvs := []string{"development", "staging", "production"}
	if len(workspace.Environments) != len(wantEnvs) {
		t.Fatalf("environments = %v, want %v", workspace.Environments, wantEnvs)
	}
	for i, env := range wantEnvs {
		if workspace.Environments[i] != env {
			t.Errorf("environment[%d] = %q, want %q", i, workspace.Environments[i], env)
		}
	}

	if workspace.Secrets.SecretsPath == "" {
		t.Error("shared secrets path was not resolved")
	}
}
