package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/envmerge"
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

// writeWorkspace lays out a temp workspace with the given files (path -> body,
// relative to the workspace root) and returns the manifest path.
func writeWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "envx.yaml")
}

// -------------------------------------------------------------------------------------

// TestResolveSecretReference verifies the wired resolver dereferences secret
// references end to end: an implicit-group reference resolves against the active
// environment's group, and an explicit shared reference resolves against it.
func TestResolveSecretReference(t *testing.T) {
	t.Parallel()

	path := writeWorkspace(t, map[string]string{
		"envx.yaml": "environments: [development, production]\n" +
			"projects:\n  api:\n    includes: [env/app]\n",
		"env/app.yaml": "password: secret://api_key\ntoken: secret://shared/token\n",
		"secrets.yaml": "secrets:\n" +
			"  development:\n    api_key: dev-secret\n" +
			"  shared:\n    token: t0k\n",
	})

	res, err := Resolve(&Input{ConfigPath: &path}, "api")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	env, err := envmerge.Build(res.Envmerge)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if v, _ := env.Get("PASSWORD"); v != "dev-secret" {
		t.Errorf("PASSWORD = %q, want dev-secret (implicit group=development)", v)
	}
	if v, _ := env.Get("TOKEN"); v != "t0k" {
		t.Errorf("TOKEN = %q, want t0k (shared group)", v)
	}
}

// -------------------------------------------------------------------------------------

// TestResolveDanglingSecretReference verifies a reference with no matching store
// entry fails loudly at Build rather than leaking the raw reference string.
func TestResolveDanglingSecretReference(t *testing.T) {
	t.Parallel()

	path := writeWorkspace(t, map[string]string{
		"envx.yaml": "environments: [development]\n" +
			"projects:\n  api:\n    includes: [env/app]\n",
		"env/app.yaml": "password: secret://development/missing\n",
		"secrets.yaml": "secrets:\n  development:\n    other: x\n",
	})

	res, err := Resolve(&Input{ConfigPath: &path}, "api")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, err := envmerge.Build(res.Envmerge); err == nil {
		t.Fatal("expected dangling reference error")
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
