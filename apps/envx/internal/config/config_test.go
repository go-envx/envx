package config

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/fixtures"
	"github.com/go-envx/envx/apps/envx/internal/schema"
)

// -------------------------------------------------------------------------------------
// fakeFlagSet is a test double for FlagSet driven by a fixed changed-set.
type fakeFlagSet struct {
	changed map[string]bool
}

// -------------------------------------------------------------------------------------
// Changed reports whether name was marked changed in the test fixture.
func (f fakeFlagSet) Changed(name string) bool { return f.changed[name] }

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
// TestResolveManifest verifies project lookup, the env precedence (flag > project
// > global), option layering, and pass-through of includes/environments into the
// engine.Config against an in-memory manifest. Terminal defaults are left to the
// engine, so an unset env stays empty here.
func TestResolveManifest(t *testing.T) {
	m := testManifest()
	none := fakeFlagSet{changed: map[string]bool{}}

	t.Run("flag wins", func(t *testing.T) {
		ec, err := resolveManifest(m, "", &Input{
			Settings: engine.Settings{Env: "from-flag"},
			Changed:  fakeFlagSet{changed: map[string]bool{schema.Env.Name: true}},
		}, "api")
		if err != nil {
			t.Fatal(err)
		}
		if ec.Settings.Env != "from-flag" {
			t.Errorf("Env = %q, want from-flag", ec.Settings.Env)
		}
	})
	t.Run("project default", func(t *testing.T) {
		ec, err := resolveManifest(m, "", &Input{Changed: none}, "api")
		if err != nil {
			t.Fatal(err)
		}
		if ec.Settings.Env != "production" {
			t.Errorf("Env = %q, want production", ec.Settings.Env)
		}
	})
	t.Run("global default", func(t *testing.T) {
		ec, err := resolveManifest(m, "", &Input{Changed: none}, "web")
		if err != nil {
			t.Fatal(err)
		}
		if ec.Settings.Env != "staging" {
			t.Errorf("Env = %q, want staging", ec.Settings.Env)
		}
	})
	t.Run("env left empty for engine default", func(t *testing.T) {
		bare := &schema.Manifest{
			Environments: []string{"development"},
			Projects: map[string]schema.Project{
				"api": {Includes: []string{"env/x"}},
			},
		}
		ec, err := resolveManifest(bare, "", &Input{Changed: none}, "api")
		if err != nil {
			t.Fatal(err)
		}
		if ec.Settings.Env != "" {
			t.Errorf("Env = %q, want empty (engine applies the default)", ec.Settings.Env)
		}
	})
	t.Run("options and includes pass through", func(t *testing.T) {
		ec, err := resolveManifest(m, "", &Input{
			Settings: engine.Settings{Prefix: "APP", Strict: true},
			Changed: fakeFlagSet{changed: map[string]bool{
				schema.Prefix.Name: true, schema.Strict.Name: true,
			}},
		}, "api")
		if err != nil {
			t.Fatal(err)
		}
		if ec.Settings.Prefix != "APP" || !ec.Settings.Strict {
			t.Errorf("options not applied: %+v", ec.Settings)
		}
		if len(ec.Includes) != 1 || ec.Includes[0] != "env/x" {
			t.Errorf("Includes = %v, want [env/x]", ec.Includes)
		}
		if len(ec.Environments) != 3 {
			t.Errorf("Environments = %v", ec.Environments)
		}
	})
	t.Run("unknown project errors", func(t *testing.T) {
		if _, err := resolveManifest(m, "", &Input{Changed: none}, "ghost"); err == nil {
			t.Error("expected error for unknown project")
		}
	})
}

// -------------------------------------------------------------------------------------
// TestResolve verifies the facade loads the manifest from the input's config path
// and resolves a known fixture project end to end.
func TestResolve(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	ec, err := Resolve(&Input{ConfigPath: &path}, "api-core")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(ec.Includes) == 0 {
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
		if got := manifestPath(&Input{ConfigPath: &flag}); got != "from-flag" {
			t.Errorf("got %q, want from-flag", got)
		}
	})
	t.Run("env when flag empty", func(t *testing.T) {
		t.Setenv(schema.Config.Env, "from-env")
		empty := ""
		if got := manifestPath(&Input{ConfigPath: &empty}); got != "from-env" {
			t.Errorf("got %q, want from-env", got)
		}
	})
	t.Run("empty when neither set", func(t *testing.T) {
		t.Setenv(schema.Config.Env, "")
		if got := manifestPath(&Input{}); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

// -------------------------------------------------------------------------------------
// TestResolveOverload verifies the overload toggle precedence (flag > default).
func TestResolveOverload(t *testing.T) {
	t.Parallel()

	changed := fakeFlagSet{changed: map[string]bool{schema.Overload.Name: true}}
	if !ResolveOverload(true, changed) {
		t.Error("expected flag value true to win")
	}
	if ResolveOverload(false, fakeFlagSet{changed: map[string]bool{}}) {
		t.Error("expected default false")
	}
}

// -------------------------------------------------------------------------------------
// TestResolveOverlayPath verifies the set action's overlay resolution: the
// default environment feeds the overlay filename, and the error paths for an
// unknown include or an undeclared environment.
func TestResolveOverlayPath(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")

	t.Run("default env and include", func(t *testing.T) {
		target, err := ResolveOverlayPath(&Input{ConfigPath: &path}, "env/postgres")
		if err != nil {
			t.Fatalf("ResolveOverlayPath: %v", err)
		}
		// The basic fixture declares [development, staging, production], so the
		// default resolves to the first declared environment.
		if filepath.Base(target) != "postgres.development.yaml" {
			t.Errorf("target = %q, want .../postgres.development.yaml", target)
		}
		if filepath.Base(filepath.Dir(target)) != "env" {
			t.Errorf("target dir = %q, want .../env", filepath.Dir(target))
		}
	})
	t.Run("unknown include errors", func(t *testing.T) {
		_, err := ResolveOverlayPath(&Input{ConfigPath: &path}, "env/ghost")
		if err == nil {
			t.Error("expected error for unknown include")
		}
	})
	t.Run("undeclared env errors", func(t *testing.T) {
		in := &Input{
			ConfigPath: &path,
			Settings:   engine.Settings{Env: "nope"},
			Changed:    fakeFlagSet{changed: map[string]bool{schema.Env.Name: true}},
		}
		_, err := ResolveOverlayPath(in, "env/postgres")
		if err == nil {
			t.Error("expected error for undeclared environment")
		}
	})
}

// -------------------------------------------------------------------------------------
// TestResolveEnv verifies the project-less env precedence (flag > manifest
// global env), leaving the terminal default to the caller.
func TestResolveEnv(t *testing.T) {
	m := testManifest()
	none := fakeFlagSet{changed: map[string]bool{}}

	t.Run("flag wins", func(t *testing.T) {
		got := ResolveEnv(
			m, "from-flag",
			fakeFlagSet{changed: map[string]bool{schema.Env.Name: true}},
		)
		if got != "from-flag" {
			t.Errorf("got %q, want from-flag", got)
		}
	})
	t.Run("manifest global when unset", func(t *testing.T) {
		if got := ResolveEnv(m, "", none); got != "staging" {
			t.Errorf("got %q, want staging", got)
		}
	})
	t.Run("empty when nothing set", func(t *testing.T) {
		bare := &schema.Manifest{Environments: []string{"development"}}
		if got := ResolveEnv(bare, "", none); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

// -------------------------------------------------------------------------------------
// TestResolverString verifies the string precedence chain.
func TestResolverString(t *testing.T) {
	t.Parallel()

	spec := schema.Prefix
	t.Run("flag wins", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "from-env", true }}
		changed := fakeFlagSet{changed: map[string]bool{spec.Name: true}}
		if got := r.String(&spec, changed, "from-flag", "layer"); got != "from-flag" {
			t.Errorf("got %q, want from-flag", got)
		}
	})
	t.Run("env wins over layers", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "from-env", true }}
		changed := fakeFlagSet{changed: map[string]bool{}}
		if got := r.String(&spec, changed, "from-flag", "layer"); got != "from-env" {
			t.Errorf("got %q, want from-env", got)
		}
	})
	t.Run("first non-empty layer", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "", false }}
		changed := fakeFlagSet{changed: map[string]bool{}}
		if got := r.String(&spec, changed, "from-flag", "", "layer2"); got != "layer2" {
			t.Errorf("got %q, want layer2", got)
		}
	})
}

// -------------------------------------------------------------------------------------
// TestResolverBool verifies the boolean precedence chain including pointer
// layers.
func TestResolverBool(t *testing.T) {
	t.Parallel()

	spec := schema.Strict
	tru := true
	t.Run("flag wins", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "false", true }}
		changed := fakeFlagSet{changed: map[string]bool{spec.Name: true}}
		if !r.Bool(&spec, changed, true, &tru) {
			t.Error("expected flag value true to win")
		}
	})
	t.Run("env parsed", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "true", true }}
		changed := fakeFlagSet{changed: map[string]bool{}}
		if !r.Bool(&spec, changed, false) {
			t.Error("expected env value true")
		}
	})
	t.Run("layer pointer", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "", false }}
		changed := fakeFlagSet{changed: map[string]bool{}}
		if !r.Bool(&spec, changed, false, nil, &tru) {
			t.Error("expected first non-nil layer true")
		}
	})
	t.Run("default false", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "", false }}
		changed := fakeFlagSet{changed: map[string]bool{}}
		if r.Bool(&spec, changed, false, nil) {
			t.Error("expected default false")
		}
	})
}
