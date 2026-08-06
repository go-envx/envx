package config

import (
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
			Prefix: strPtr("APP"), RequireOverlays: boolPtr(true),
		})
		if err != nil {
			t.Fatal(err)
		}
		if r.Envmerge.Settings.Prefix != "APP" || !r.Envmerge.Settings.RequireOverlays {
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

// TestResolveProject verifies ResolveProject loads the manifest from the input's
// config path and resolves a known fixture project end to end.
func TestResolveProject(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	r, err := ResolveProject(&Input{ConfigPath: &path}, "api-core")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if len(r.Envmerge.Includes) == 0 {
		t.Error("expected includes from the fixture project")
	}
}

// -------------------------------------------------------------------------------------

// TestResolveWorkspace verifies ResolveWorkspace surfaces the resolved secrets
// store location as data, resolves no project, and never wires a value resolver —
// constructing the manager and opening the store are ResolveProject's jobs.
func TestResolveWorkspace(t *testing.T) {
	t.Parallel()

	// Default: secrets.yaml beside the manifest, no project.
	base := fixtures.Manifest("basic")
	r, err := ResolveWorkspace(&Input{ConfigPath: &base})
	if err != nil {
		t.Fatalf("ResolveWorkspace basic: %v", err)
	}
	if filepath.Base(r.Secrets.SecretsPath) != "secrets.yaml" {
		t.Errorf(
			"Secrets.SecretsPath = %q, want .../secrets.yaml",
			r.Secrets.SecretsPath,
		)
	}
	wantDefaultKeys := filepath.Join(filepath.Dir(r.Secrets.SecretsPath), "envx.keys")
	if r.Secrets.KeysPath != wantDefaultKeys {
		t.Errorf("Secrets.KeysPath = %q, want %q", r.Secrets.KeysPath, wantDefaultKeys)
	}
	if r.Envmerge.ValueResolver != nil {
		t.Error("ResolveWorkspace must not wire a value resolver")
	}
	if len(r.Envmerge.Includes) != 0 {
		t.Error("ResolveWorkspace resolves no project, so it has no includes")
	}

	// A workspace secrets path flows through to the secrets input.
	m := testManifest()
	m.Secrets.SecretsPath = "private/secrets.yaml"
	dir := t.TempDir()
	r2, err := resolveManifest(manifestContext{manifest: m, dir: dir}, &Input{})
	if err != nil {
		t.Fatalf("resolveManifest secrets path: %v", err)
	}
	wantPath := filepath.Join(dir, "private", "secrets.yaml")
	if r2.Secrets.SecretsPath != wantPath {
		t.Errorf("Secrets.SecretsPath = %q, want %q", r2.Secrets.SecretsPath, wantPath)
	}
	wantKeysPath := filepath.Join(dir, "private", "envx.keys")
	if r2.Secrets.KeysPath != wantKeysPath {
		t.Errorf("Secrets.KeysPath = %q, want %q", r2.Secrets.KeysPath, wantKeysPath)
	}

	// An explicit relative key path is resolved against the manifest directory,
	// not against the custom secrets store directory.
	m.Secrets.KeysPath = "keys/envx.keys"
	r3, err := resolveManifest(manifestContext{manifest: m, dir: dir}, &Input{})
	if err != nil {
		t.Fatalf("resolveManifest relative keys path: %v", err)
	}
	wantRelativeKeysPath := filepath.Join(dir, "keys", "envx.keys")
	if r3.Secrets.KeysPath != wantRelativeKeysPath {
		t.Errorf(
			"Secrets.KeysPath = %q, want %q",
			r3.Secrets.KeysPath,
			wantRelativeKeysPath,
		)
	}

	// An explicit absolute key path remains rooted at its own location.
	absoluteKeysPath := filepath.Join(t.TempDir(), "envx.keys")
	m.Secrets.KeysPath = absoluteKeysPath
	r4, err := resolveManifest(manifestContext{manifest: m, dir: dir}, &Input{})
	if err != nil {
		t.Fatalf("resolveManifest absolute keys path: %v", err)
	}
	if r4.Secrets.KeysPath != absoluteKeysPath {
		t.Errorf(
			"Secrets.KeysPath = %q, want %q",
			r4.Secrets.KeysPath,
			absoluteKeysPath,
		)
	}
}

// -------------------------------------------------------------------------------------

// TestResolveProjectResolvesSecretReference verifies ResolveProject wires the
// resolver so secret references dereference end to end: explicit environment and
// shared groups both resolve against the store.
func TestResolveProjectResolvesSecretReference(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("resolve/secret-reference")
	res, err := ResolveProject(&Input{ConfigPath: &path}, "api")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	env, err := envmerge.Build(res.Envmerge)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if v, _ := env.Get("PASSWORD"); v != "dev-secret" {
		t.Errorf("PASSWORD = %q, want dev-secret (development group)", v)
	}
	if v, _ := env.Get("TOKEN"); v != "t0k" {
		t.Errorf("TOKEN = %q, want t0k (shared group)", v)
	}
}

// TestResolveWorkspaceIgnoresSecretsStore verifies a workspace resolution skips
// the secrets store entirely — it neither reads a malformed store nor wires a
// resolver.
func TestResolveWorkspaceIgnoresSecretsStore(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("resolve/global-ignores-store")
	res, err := ResolveWorkspace(&Input{ConfigPath: &path})
	if err != nil {
		t.Fatalf("ResolveWorkspace: %v", err)
	}
	if res.Envmerge.ValueResolver != nil {
		t.Error("workspace resolution should not construct a value resolver")
	}
}

// -------------------------------------------------------------------------------------

// TestResolveProjectDanglingSecretReference verifies a reference with no matching
// store entry fails loudly at Build rather than leaking the raw reference string.
func TestResolveProjectDanglingSecretReference(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("resolve/dangling-reference")
	res, err := ResolveProject(&Input{ConfigPath: &path}, "api")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
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
