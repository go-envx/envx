package config

import (
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/go-envx/envx/apps/envx/internal/manifest"
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
func testManifest() *manifest.Manifest {
	return &manifest.Manifest{
		Environments: []string{"development", "staging", "production"},
		Settings:     manifest.Settings{Env: "staging"},
		Projects: map[string]manifest.Project{
			"api": {
				Includes: []string{"env/x"},
				Settings: manifest.Settings{Env: "production"},
			},
			"web": {Includes: []string{"env/y"}},
		},
	}
}

// -------------------------------------------------------------------------------------
// TestResolve verifies project lookup, the env precedence (flag > project >
// global), option layering, and pass-through of includes/environments into the
// engine.Config. Terminal defaults are left to the engine, so an unset env stays
// empty here.
func TestResolve(t *testing.T) {
	m := testManifest()
	none := fakeFlagSet{changed: map[string]bool{}}

	t.Run("flag wins", func(t *testing.T) {
		ec, err := Resolve(
			m, "api", engine.Settings{Env: "from-flag"},
			fakeFlagSet{changed: map[string]bool{flags.Env.Name: true}},
		)
		if err != nil {
			t.Fatal(err)
		}
		if ec.Settings.Env != "from-flag" {
			t.Errorf("Env = %q, want from-flag", ec.Settings.Env)
		}
	})
	t.Run("project default", func(t *testing.T) {
		ec, err := Resolve(m, "api", engine.Settings{}, none)
		if err != nil {
			t.Fatal(err)
		}
		if ec.Settings.Env != "production" {
			t.Errorf("Env = %q, want production", ec.Settings.Env)
		}
	})
	t.Run("global default", func(t *testing.T) {
		ec, err := Resolve(m, "web", engine.Settings{}, none)
		if err != nil {
			t.Fatal(err)
		}
		if ec.Settings.Env != "staging" {
			t.Errorf("Env = %q, want staging", ec.Settings.Env)
		}
	})
	t.Run("env left empty for engine default", func(t *testing.T) {
		bare := &manifest.Manifest{
			Environments: []string{"development"},
			Projects: map[string]manifest.Project{
				"api": {Includes: []string{"env/x"}},
			},
		}
		ec, err := Resolve(bare, "api", engine.Settings{}, none)
		if err != nil {
			t.Fatal(err)
		}
		if ec.Settings.Env != "" {
			t.Errorf("Env = %q, want empty (engine applies the default)", ec.Settings.Env)
		}
	})
	t.Run("options and includes pass through", func(t *testing.T) {
		ec, err := Resolve(
			m, "api", engine.Settings{Prefix: "APP", Strict: true},
			fakeFlagSet{changed: map[string]bool{
				flags.Prefix.Name: true, flags.Strict.Name: true,
			}},
		)
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
		if _, err := Resolve(m, "ghost", engine.Settings{}, none); err == nil {
			t.Error("expected error for unknown project")
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
			fakeFlagSet{changed: map[string]bool{flags.Env.Name: true}},
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
		bare := &manifest.Manifest{Environments: []string{"development"}}
		if got := ResolveEnv(bare, "", none); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

// -------------------------------------------------------------------------------------
// TestResolverString verifies the string precedence chain.
func TestResolverString(t *testing.T) {
	t.Parallel()

	spec := flags.Prefix
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

	spec := flags.Strict
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
