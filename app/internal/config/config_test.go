package config

import (
	"testing"

	"github.com/go-envx/envx/app/internal/fixtures"
	"github.com/go-envx/envx/app/internal/schema"
)

// -------------------------------------------------------------------------------------

// strPtr returns a pointer to s, for building optional Input values in tests.
func strPtr(s string) *string { return &s }

// -------------------------------------------------------------------------------------

// boolPtr returns a pointer to b, for building optional Input values in tests.
func boolPtr(b bool) *bool { return &b }

// -------------------------------------------------------------------------------------

// testManifest builds an in-memory manifest with global and project-level env
// settings for exercising the precedence chain.
func testManifest() *schema.Manifest {
	return &schema.Manifest{
		Environments: []string{"development", "staging", "production"},
		Settings:     schema.Settings{Env: "staging"},
		Projects: map[string]schema.Project{
			"api": {
				Includes: []string{"env/x"},
				Settings: schema.Settings{Env: "production"},
			},
			"web": {Includes: []string{"env/y"}},
		},
	}
}

// -------------------------------------------------------------------------------------

// TestResolveManifest verifies project lookup, the env precedence (explicit >
// project > global), setting layering, and pass-through of includes/environments
// into the envmerge.Params against an in-memory manifest. An empty project
// resolves the global context only. Terminal defaults are left to envmerge, so an
// unset env stays empty here.
func TestResolveManifest(t *testing.T) {
	m := testManifest()

	t.Run("explicit wins", func(t *testing.T) {
		r, err := resolveManifest(
			manifestContext{manifest: m, project: "api"},
			&Input{Env: strPtr("from-flag")},
		)
		if err != nil {
			t.Fatal(err)
		}
		if r.Envmerge.Settings.Env != "from-flag" {
			t.Errorf("Env = %q, want from-flag", r.Envmerge.Settings.Env)
		}
	})
	t.Run("project default", func(t *testing.T) {
		r, err := resolveManifest(manifestContext{manifest: m, project: "api"}, &Input{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Envmerge.Settings.Env != "production" {
			t.Errorf("Env = %q, want production", r.Envmerge.Settings.Env)
		}
	})
	t.Run("global default", func(t *testing.T) {
		r, err := resolveManifest(manifestContext{manifest: m, project: "web"}, &Input{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Envmerge.Settings.Env != "staging" {
			t.Errorf("Env = %q, want staging", r.Envmerge.Settings.Env)
		}
	})
	t.Run("env left empty for envmerge default", func(t *testing.T) {
		bare := &schema.Manifest{
			Environments: []string{"development"},
			Projects: map[string]schema.Project{
				"api": {Includes: []string{"env/x"}},
			},
		}
		r, err := resolveManifest(manifestContext{manifest: bare, project: "api"}, &Input{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Envmerge.Settings.Env != "" {
			t.Errorf(
				"Env = %q, want empty (envmerge applies the default)",
				r.Envmerge.Settings.Env,
			)
		}
	})
	t.Run("settings and includes pass through", func(t *testing.T) {
		r, err := resolveManifest(manifestContext{manifest: m, project: "api"}, &Input{
			Prefix: strPtr("APP"), Strict: boolPtr(true),
		})
		if err != nil {
			t.Fatal(err)
		}
		if r.Envmerge.Settings.Prefix != "APP" || !r.Envmerge.Settings.Strict {
			t.Errorf("settings not applied: %+v", r.Envmerge.Settings)
		}
		if len(r.Envmerge.Includes) != 1 || r.Envmerge.Includes[0] != "env/x" {
			t.Errorf("Includes = %v, want [env/x]", r.Envmerge.Includes)
		}
		if len(r.Envmerge.Environments) != 3 {
			t.Errorf("Environments = %v", r.Envmerge.Environments)
		}
	})
	t.Run("delimiter explicit flows through", func(t *testing.T) {
		r, err := resolveManifest(manifestContext{manifest: m, project: "api"}, &Input{
			Delimiter: strPtr("|"),
		})
		if err != nil {
			t.Fatal(err)
		}
		if r.Envmerge.Settings.Delimiter != "|" {
			t.Errorf("Delimiter = %q, want |", r.Envmerge.Settings.Delimiter)
		}
	})
	t.Run("unknown project errors", func(t *testing.T) {
		_, err := resolveManifest(manifestContext{manifest: m, project: "ghost"}, &Input{})
		if err == nil {
			t.Error("expected error for unknown project")
		}
	})
	t.Run("empty project resolves global only", func(t *testing.T) {
		r, err := resolveManifest(manifestContext{manifest: m, project: ""}, &Input{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Envmerge.Settings.Env != "staging" {
			t.Errorf("Env = %q, want staging (global)", r.Envmerge.Settings.Env)
		}
		if len(r.Envmerge.Includes) != 0 {
			t.Errorf("Includes = %v, want empty for no project", r.Envmerge.Includes)
		}
	})
}

// -------------------------------------------------------------------------------------

// TestOverloadResolution verifies overload now layers through the manifest
// (explicit > project > global), the precedence that was previously dropped.
func TestOverloadResolution(t *testing.T) {
	t.Parallel()

	manifestWith := func(global, project *bool) *schema.Manifest {
		return &schema.Manifest{
			Environments: []string{"development"},
			Settings:     schema.Settings{Overload: global},
			Projects: map[string]schema.Project{
				"api": {
					Includes: []string{"env/x"},
					Settings: schema.Settings{Overload: project},
				},
			},
		}
	}

	t.Run("explicit wins over manifest", func(t *testing.T) {
		r, err := resolveManifest(
			manifestContext{manifest: manifestWith(boolPtr(false), nil), project: "api"},
			&Input{Overload: boolPtr(true)},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Runner.Overload {
			t.Error("explicit overload true should win")
		}
	})
	t.Run("manifest global layer honored", func(t *testing.T) {
		r, err := resolveManifest(
			manifestContext{manifest: manifestWith(boolPtr(true), nil), project: "api"},
			&Input{},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Runner.Overload {
			t.Error("manifest global overload=true should be honored")
		}
	})
	t.Run("project layer over global", func(t *testing.T) {
		r, err := resolveManifest(
			manifestContext{
				manifest: manifestWith(boolPtr(false), boolPtr(true)),
				project:  "api",
			},
			&Input{},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Runner.Overload {
			t.Error("project overload=true should win over global false")
		}
	})
}

// -------------------------------------------------------------------------------------

// TestResolve verifies the facade loads the manifest from the input's config path
// and resolves a known fixture project end to end.
func TestResolve(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	r, err := Resolve(&Input{ConfigPath: &path}, "api-core")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(r.Envmerge.Includes) == 0 {
		t.Error("expected includes from the fixture project")
	}
}

// -------------------------------------------------------------------------------------

// TestManifestPath verifies the manifest-location precedence: --config flag wins,
// then ENVX_CONFIG, then empty (which defers to the manifest walk-up).
func TestManifestPath(t *testing.T) {
	t.Run("flag wins over env", func(t *testing.T) {
		t.Setenv(schema.Config.Env, "from-env")
		flag := "from-flag"
		if got := resolveManifestPath(&Input{ConfigPath: &flag}); got != "from-flag" {
			t.Errorf("got %q, want from-flag", got)
		}
	})
	t.Run("env when flag empty", func(t *testing.T) {
		t.Setenv(schema.Config.Env, "from-env")
		empty := ""
		if got := resolveManifestPath(&Input{ConfigPath: &empty}); got != "from-env" {
			t.Errorf("got %q, want from-env", got)
		}
	})
	t.Run("empty when neither set", func(t *testing.T) {
		t.Setenv(schema.Config.Env, "")
		if got := resolveManifestPath(&Input{}); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}
