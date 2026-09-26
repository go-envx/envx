package workspace_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/features/workspace"
)

// testWorkspace builds an in-memory workspace entity for exercising pure query
// methods without any file I/O.
func testWorkspace() *workspace.Workspace {
	return &workspace.Workspace{
		Environments: []string{"development", "staging", "production"},
		Projects: map[string]workspace.Project{
			"api": {Includes: []string{"env/postgres", "apps/api/env/api"}},
			"web": {Includes: []string{"env/web"}},
		},
	}
}

// TestLookupProject verifies a known project resolves to its definition and an
// unknown project reports not found.
func TestLookupProject(t *testing.T) {
	t.Parallel()

	ws := testWorkspace()

	proj, ok := ws.LookupProject("api")
	if !ok {
		t.Fatal("expected project api to be found")
	}
	if len(proj.Includes) != 2 {
		t.Errorf("Includes = %v, want 2 entries", proj.Includes)
	}

	if _, ok := ws.LookupProject("ghost"); ok {
		t.Error("expected unknown project to report not found")
	}
}

// TestDefaultEnvironment verifies the default resolves to the first declared
// environment and that an empty list yields "".
func TestDefaultEnvironment(t *testing.T) {
	t.Parallel()

	ws := &workspace.Workspace{Environments: []string{"staging", "development"}}
	if got := ws.DefaultEnvironment(); got != "staging" {
		t.Errorf("DefaultEnvironment = %q, want staging", got)
	}
	if got := (&workspace.Workspace{}).DefaultEnvironment(); got != "" {
		t.Errorf("DefaultEnvironment() = %q, want empty", got)
	}
}

// TestHasEnvironment verifies declared environments are recognized and an
// undeclared one is rejected.
func TestHasEnvironment(t *testing.T) {
	t.Parallel()

	ws := testWorkspace()

	for _, env := range []string{"development", "staging", "production"} {
		if !ws.HasEnvironment(env) {
			t.Errorf("HasEnvironment(%q) = false, want true", env)
		}
	}
	if ws.HasEnvironment("qa") {
		t.Error(`HasEnvironment("qa") = true, want false`)
	}
}

// TestHasInclude verifies an include declared by any project is found (across
// projects) and an undeclared include is not.
func TestHasInclude(t *testing.T) {
	t.Parallel()

	ws := testWorkspace()

	for _, inc := range []string{"env/postgres", "apps/api/env/api", "env/web"} {
		if !ws.HasInclude(inc) {
			t.Errorf("HasInclude(%q) = false, want true", inc)
		}
	}
	if ws.HasInclude("env/ghost") {
		t.Error(`HasInclude("env/ghost") = true, want false`)
	}
}

// TestValidate verifies the structural constraints: a well-formed workspace
// passes, while a missing environment/project, an absent include list, or an
// empty include entry each report an error.
func TestValidate(t *testing.T) {
	t.Parallel()

	if err := testWorkspace().Validate(); err != nil {
		t.Errorf("Validate() on a well-formed workspace: %v", err)
	}

	tests := map[string]*workspace.Workspace{
		"no environments": {
			Projects: map[string]workspace.Project{"api": {Includes: []string{"env/x"}}},
		},
		"no projects": {
			Environments: []string{"development"},
		},
		"empty include": {
			Environments: []string{"development"},
			Projects:     map[string]workspace.Project{"api": {Includes: []string{""}}},
		},
		"no includes": {
			Environments: []string{"development"},
			Projects:     map[string]workspace.Project{"api": {Includes: []string{}}},
		},
		"invalid severity": {
			Environments: []string{"development"},
			Projects: map[string]workspace.Project{
				"api": {Includes: []string{"env/x"}},
			},
			ValidateSeverities: map[string]string{
				"unknown_check": "invalid_severity",
			},
		},
	}
	for name, ws := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := ws.Validate(); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}
