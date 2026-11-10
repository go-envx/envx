package env

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/core/workspace"
)

// appInclude is the absolute include path of the fixture project.
const appInclude = "/ws/env/app"

// testWorkspace declares one project "app" with development and production.
func testWorkspace() workspace.Workspace {
	return workspace.Workspace{
		Root:         "/ws",
		Environments: []string{"development", "production"},
		Projects: map[string]workspace.Project{
			"app": {Includes: []string{"env/app"}, IncludePaths: []string{appInclude}},
		},
	}
}

// newService constructs a service, failing the test on error.
//
//nolint:gocritic // Test fixture mirrors New's signature.
func newService(t *testing.T, ws workspace.Workspace, deps Dependencies) *Service {
	t.Helper()
	s, err := New(ws, deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T {
	return &v
}

// TestNewValidation verifies required dependencies and the reference pattern
// are checked at construction.
func TestNewValidation(t *testing.T) {
	t.Parallel()

	if _, err := New(testWorkspace(), Dependencies{}); err == nil {
		t.Error("New without EnvNamespaces succeeded")
	}
	ws := testWorkspace()
	ws.Settings.ReferencePattern = ptr("(unterminated")
	if _, err := New(ws, Dependencies{EnvNamespaces: newFakeNamespaces()}); err == nil {
		t.Error("New with an invalid reference pattern succeeded")
	}
	ws = testWorkspace()
	ws.Projects["app"] = workspace.Project{
		Settings: workspace.Settings{ReferencePattern: ptr("(")},
	}
	if _, err := New(ws, Dependencies{EnvNamespaces: newFakeNamespaces()}); err == nil {
		t.Error("New with an invalid project reference pattern succeeded")
	}
}

// TestNewPerformsNoIO verifies construction reads no namespaces and opens no
// resolver.
func TestNewPerformsNoIO(t *testing.T) {
	t.Parallel()

	namespaces := newFakeNamespaces()
	opener := &fakeOpener{}
	newService(t, testWorkspace(), Dependencies{
		EnvNamespaces:    namespaces,
		NewValueResolver: opener.open,
	})
	if namespaces.baseReads != 0 || opener.calls != 0 {
		t.Errorf("reads = %d, opens = %d; want none", namespaces.baseReads, opener.calls)
	}
}

// TestNewCopiesWorkspace verifies later caller mutation does not change the
// service.
func TestNewCopiesWorkspace(t *testing.T) {
	t.Parallel()

	namespaces := newFakeNamespaces().
		base(t, appInclude, "host: base").
		overlay(t, appInclude, "development", "host: dev")
	ws := testWorkspace()
	ws.Settings.Prefix = ptr("app")
	s := newService(t, ws, Dependencies{EnvNamespaces: namespaces})

	ws.Environments[0] = "production"
	*ws.Settings.Prefix = "other"
	ws.Projects["app"].IncludePaths[0] = "/elsewhere"

	got, err := s.Get(GetParams{Key: "APP_HOST"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Value != "dev" {
		t.Errorf("Value = %q, want dev", got.Value)
	}
}

// TestProjectSelection verifies an empty project selects the only project and
// unknown or ambiguous selections fail.
func TestProjectSelection(t *testing.T) {
	t.Parallel()

	namespaces := newFakeNamespaces().
		base(t, appInclude, "host: app").
		base(t, "/ws/env/web", "host: web")
	single := newService(t, testWorkspace(), Dependencies{EnvNamespaces: namespaces})
	if got, err := single.Get(GetParams{Key: "host"}); err != nil || got.Value != "app" {
		t.Errorf("Get(only project) = %q, %v; want app", got.Value, err)
	}
	_, err := single.Get(GetParams{Query: Query{Project: "missing"}, Key: "host"})
	if !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("unknown project error = %v, want ErrProjectNotFound", err)
	}

	ws := testWorkspace()
	ws.Projects["web"] = workspace.Project{IncludePaths: []string{"/ws/env/web"}}
	multi := newService(t, ws, Dependencies{EnvNamespaces: namespaces})
	if _, err := multi.Get(GetParams{Key: "host"}); !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("ambiguous project error = %v, want ErrProjectNotFound", err)
	}
	got, err := multi.Get(GetParams{Query: Query{Project: "web"}, Key: "host"})
	if err != nil || got.Value != "web" {
		t.Errorf("Get(web) = %q, %v; want web", got.Value, err)
	}
}

// TestEnvironmentPrecedence verifies overrides, project, and workspace
// settings select the environment before the first declared one.
func TestEnvironmentPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// name labels the case.
		name string
		// override is the query's environment override.
		override *string
		// project is the project's environment setting.
		project *string
		// global is the workspace environment setting.
		global *string
		// want is the expected overlay value.
		want string
		// wantErr is the expected error.
		wantErr error
	}{
		{name: "first declared", want: "dev"},
		{name: "global", global: ptr("production"), want: "prod"},
		{
			name: "project over global", project: ptr("production"),
			global: ptr("development"), want: "prod",
		},
		{
			name: "override over project", override: ptr("development"),
			project: ptr("production"), want: "dev",
		},
		{name: "empty override ignored", override: ptr(""), want: "dev"},
		{
			name: "undeclared", override: ptr("staging"),
			wantErr: ErrEnvironmentNotDeclared,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			namespaces := newFakeNamespaces().
				base(t, appInclude, "host: base").
				overlay(t, appInclude, "development", "host: dev").
				overlay(t, appInclude, "production", "host: prod")
			ws := testWorkspace()
			ws.Settings.Env = tt.global
			ws.Projects["app"] = workspace.Project{
				IncludePaths: []string{appInclude},
				Settings:     workspace.Settings{Env: tt.project},
			}
			s := newService(t, ws, Dependencies{EnvNamespaces: namespaces})

			got, err := s.Get(GetParams{
				Query: Query{Options: workspace.Settings{Env: tt.override}},
				Key:   "host",
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Get error = %v, want %v", err, tt.wantErr)
			}
			if got.Value != tt.want {
				t.Errorf("Value = %q, want %q", got.Value, tt.want)
			}
		})
	}
}

// TestSettingsPrecedence verifies overrides win even when empty, then project,
// then workspace settings.
func TestSettingsPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// name labels the case.
		name string
		// override is the query's prefix override.
		override *string
		// project is the project's prefix.
		project *string
		// global is the workspace prefix.
		global *string
		// want is the expected key.
		want string
	}{
		{name: "none", want: "HOST"},
		{name: "global", global: ptr("g"), want: "G_HOST"},
		{name: "project", project: ptr("p"), global: ptr("g"), want: "P_HOST"},
		{name: "override", override: ptr("o"), project: ptr("p"), want: "O_HOST"},
		{name: "empty override", override: ptr(""), project: ptr("p"), want: "HOST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ws := testWorkspace()
			ws.Settings.Prefix = tt.global
			ws.Projects["app"] = workspace.Project{
				IncludePaths: []string{appInclude},
				Settings:     workspace.Settings{Prefix: tt.project},
			}
			s := newService(t, ws, Dependencies{
				EnvNamespaces: newFakeNamespaces().base(t, appInclude, "host: x"),
			})
			result, err := s.Explain(ExplainParams{
				Query: Query{Options: workspace.Settings{Prefix: tt.override}},
			})
			if err != nil {
				t.Fatalf("Explain: %v", err)
			}
			if len(result.Entries) != 1 || result.Entries[0].Key != tt.want {
				t.Errorf("Entries = %+v, want key %s", result.Entries, tt.want)
			}
		})
	}
}

// TestProjectReferencePattern verifies a project pattern overrides the
// workspace syntax for that operation.
func TestProjectReferencePattern(t *testing.T) {
	t.Parallel()

	ws := testWorkspace()
	ws.Projects["app"] = workspace.Project{
		IncludePaths: []string{appInclude},
		Settings:     workspace.Settings{ReferencePattern: ptr(`\$\{([^}]*)\}`)},
	}
	s := newService(t, ws, Dependencies{
		EnvNamespaces: newFakeNamespaces().base(t, appInclude, `
			host: db
			url: ${HOST}/{{HOST}}
		`),
	})
	got, err := s.Get(GetParams{Key: "url", Reveal: true})
	if err != nil || got.Value != "db/{{HOST}}" {
		t.Errorf("Get = %q, %v; want db/{{HOST}}", got.Value, err)
	}
}
