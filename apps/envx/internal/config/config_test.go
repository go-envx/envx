package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/fixtures"
	"github.com/go-envx/envx/apps/envx/internal/flags"
)

// -------------------------------------------------------------------------------------
// fakeFlagSet is a test double for config.FlagSet driven by a fixed changed-set.
type fakeFlagSet struct {
	changed map[string]bool
}

// -------------------------------------------------------------------------------------
// Changed reports whether name was marked changed in the test fixture.
func (f fakeFlagSet) Changed(name string) bool { return f.changed[name] }

// -------------------------------------------------------------------------------------
// writeManifest writes a manifest file into a fresh temp dir and returns its
// path.
func writeManifest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "envx.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// -------------------------------------------------------------------------------------
// TestLoadValid verifies a well-formed manifest parses with its dir recorded.
func TestLoadValid(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, `
environments: [development, production]
projects:
  api:
    includes:
      - env/postgres
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Dir() != filepath.Dir(path) {
		t.Errorf("Dir() = %q, want %q", cfg.Dir(), filepath.Dir(path))
	}
	if !cfg.HasEnvironment("production") {
		t.Error("expected production environment to be present")
	}
	if _, ok := cfg.LookupProject("api"); !ok {
		t.Error("expected project api to be present")
	}
}

// -------------------------------------------------------------------------------------
// TestLoadInvalid verifies structural validation rejects malformed manifests.
func TestLoadInvalid(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"no environments": "projects:\n  api:\n    includes: [env/x]\n",
		"no projects":     "environments: [development]\n",
		"empty include": "environments: [development]\n" +
			"projects:\n  api:\n    includes: [\"\"]\n",
		"no includes": "environments: [development]\n" +
			"projects:\n  api:\n    includes: []\n",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := Load(writeManifest(t, body)); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestLookupInclude verifies an include path resolves to its absolute dir and
// base name.
func TestLookupInclude(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, `
environments: [development]
projects:
  api:
    includes:
      - apps/api/env/api
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	dir, name, ok := cfg.LookupInclude("apps/api/env/api")
	if !ok {
		t.Fatal("expected include to be found")
	}
	if name != "api" {
		t.Errorf("name = %q, want api", name)
	}
	wantDir := filepath.Join(cfg.Dir(), "apps", "api", "env")
	if dir != wantDir {
		t.Errorf("dir = %q, want %q", dir, wantDir)
	}
}

// -------------------------------------------------------------------------------------
// TestDiscoverExplicit verifies an explicit path is honored and a missing path
// is an error.
func TestDiscoverExplicit(t *testing.T) {
	t.Parallel()

	got, err := Discover(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}

	if _, err := Discover(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("expected error for missing explicit path")
	}
}

// -------------------------------------------------------------------------------------
// TestDiscoverWalkUp verifies that with no explicit path or ENVX_CONFIG the
// search walks up from the working directory to the nearest envx.yaml.
func TestDiscoverWalkUp(t *testing.T) {
	// t.Chdir and t.Setenv forbid t.Parallel.
	t.Setenv(flags.Config.Env, "")

	manifest := fixtures.Manifest("basic")
	t.Chdir(filepath.Join(filepath.Dir(manifest), "apps", "api-core", "env"))

	got, err := Discover("")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != manifest {
		t.Errorf("Discover() = %q, want %q", got, manifest)
	}
}

// -------------------------------------------------------------------------------------
// TestDiscoverEnvVar verifies ENVX_CONFIG is honored when no explicit path is
// supplied.
func TestDiscoverEnvVar(t *testing.T) {
	manifest := fixtures.Manifest("basic")
	t.Setenv(flags.Config.Env, manifest)

	got, err := Discover("")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != manifest {
		t.Errorf("Discover() = %q, want %q", got, manifest)
	}
}

// -------------------------------------------------------------------------------------
// TestResolveLoadsManifest verifies Resolve discovers and loads the manifest
// into the shared root context. Environment selection is no longer part of the
// root context — each action resolves its own target environment.
func TestResolveLoadsManifest(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, `
environments: [development, staging, production]
settings:
  env: staging
projects:
  api:
    includes: [env/x]
`)
	g, err := Resolve(path)
	if err != nil {
		t.Fatal(err)
	}
	if g.Config == nil {
		t.Fatal("expected manifest to be loaded")
	}
	if g.ConfigPath != path {
		t.Errorf("ConfigPath = %q, want %q", g.ConfigPath, path)
	}
	if g.Config.Settings.Env != "staging" {
		t.Errorf("Settings.Env = %q, want staging", g.Config.Settings.Env)
	}
}

// -------------------------------------------------------------------------------------
// TestResolverString verifies the string precedence chain.
func TestResolverString(t *testing.T) {
	t.Parallel()

	spec := flags.Prefix
	t.Run("flag wins", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "from-env", true }}
		changed := fakeFlagSet{changed: map[string]bool{spec.Name: true}}
		if got := r.String(spec, changed, "from-flag", "layer"); got != "from-flag" {
			t.Errorf("got %q, want from-flag", got)
		}
	})
	t.Run("env wins over layers", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "from-env", true }}
		changed := fakeFlagSet{changed: map[string]bool{}}
		if got := r.String(spec, changed, "from-flag", "layer"); got != "from-env" {
			t.Errorf("got %q, want from-env", got)
		}
	})
	t.Run("first non-empty layer", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "", false }}
		changed := fakeFlagSet{changed: map[string]bool{}}
		if got := r.String(spec, changed, "from-flag", "", "layer2"); got != "layer2" {
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
		if !r.Bool(spec, changed, true, &tru) {
			t.Error("expected flag value true to win")
		}
	})
	t.Run("env parsed", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "true", true }}
		changed := fakeFlagSet{changed: map[string]bool{}}
		if !r.Bool(spec, changed, false) {
			t.Error("expected env value true")
		}
	})
	t.Run("layer pointer", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "", false }}
		changed := fakeFlagSet{changed: map[string]bool{}}
		if !r.Bool(spec, changed, false, nil, &tru) {
			t.Error("expected first non-nil layer true")
		}
	})
	t.Run("default false", func(t *testing.T) {
		r := &Resolver{LookupEnv: func(string) (string, bool) { return "", false }}
		changed := fakeFlagSet{changed: map[string]bool{}}
		if r.Bool(spec, changed, false, nil) {
			t.Error("expected default false")
		}
	})
}
