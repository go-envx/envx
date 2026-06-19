package actions

import (
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/spf13/cobra"
)

// -------------------------------------------------------------------------------------
// changeSet is a config.FlagSet test double driven by a fixed changed-set.
type changeSet map[string]bool

// -------------------------------------------------------------------------------------
// Changed reports whether name was marked changed in the fixture.
func (c changeSet) Changed(name string) bool { return c[name] }

// -------------------------------------------------------------------------------------
// TestRegisterEngineFlags verifies the engine flag group is bound onto a command
// and that parsing writes through to the destination struct.
func TestRegisterEngineFlags(t *testing.T) {
	t.Parallel()

	var dst engine.Settings
	cmd := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error {
		return nil
	}}
	RegisterEngineFlags(cmd, &dst)

	for _, name := range []string{
		flags.Strict.Name, flags.Prefix.Name, flags.Suffix.Name,
		flags.NamespacePrefix.Name,
	} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag %q was not registered", name)
		}
	}

	cmd.SetArgs([]string{
		"--strict", "--prefix", "APP", "--suffix", "V2", "--namespace-prefix",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !dst.Strict || !dst.NamespacePrefix {
		t.Errorf("bool flags not applied: %+v", dst)
	}
	if dst.Prefix != "APP" || dst.Suffix != "V2" {
		t.Errorf("string flags not applied: %+v", dst)
	}
}

// -------------------------------------------------------------------------------------
// TestRegisterEnvFlag verifies the --env flag is bound onto a command and that
// parsing writes through to the destination.
func TestRegisterEnvFlag(t *testing.T) {
	t.Parallel()

	var env string
	cmd := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error {
		return nil
	}}
	RegisterEnvFlag(cmd, &env)

	if cmd.Flags().Lookup(flags.Env.Name) == nil {
		t.Fatalf("flag %q was not registered", flags.Env.Name)
	}
	cmd.SetArgs([]string{"--env", "production"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if env != "production" {
		t.Errorf("env = %q, want production", env)
	}
}

// -------------------------------------------------------------------------------------
// TestResolveSettings verifies the env precedence (flag > project > global >
// default) and that the merge options are layered through to the result.
func TestResolveSettings(t *testing.T) {
	cfg := &config.Config{
		Environments: []string{"development", "staging", "production"},
		Settings:     config.Settings{Env: "staging"},
		Projects: map[string]config.Project{
			"api": {
				Includes: []string{"env/x"},
				Settings: config.Settings{Env: "production"},
			},
			"web": {Includes: []string{"env/y"}},
		},
	}
	g := &config.Global{Config: cfg}
	none := changeSet{}

	t.Run("flag wins", func(t *testing.T) {
		got := ResolveSettings(
			g, "api", engine.Settings{Env: "from-flag"},
			changeSet{flags.Env.Name: true},
		)
		if got.Env != "from-flag" {
			t.Errorf("Env = %q, want from-flag", got.Env)
		}
	})
	t.Run("project default", func(t *testing.T) {
		got := ResolveSettings(g, "api", engine.Settings{}, none)
		if got.Env != "production" {
			t.Errorf("Env = %q, want production", got.Env)
		}
	})
	t.Run("global default", func(t *testing.T) {
		got := ResolveSettings(g, "web", engine.Settings{}, none)
		if got.Env != "staging" {
			t.Errorf("Env = %q, want staging", got.Env)
		}
	})
	t.Run("development fallback", func(t *testing.T) {
		bare := &config.Global{Config: &config.Config{
			Environments: []string{"development"},
			Projects: map[string]config.Project{
				"api": {Includes: []string{"env/x"}},
			},
		}}
		got := ResolveSettings(bare, "api", engine.Settings{}, none)
		if got.Env != "development" {
			t.Errorf("Env = %q, want development", got.Env)
		}
	})
	t.Run("options layered through", func(t *testing.T) {
		got := ResolveSettings(
			g, "api", engine.Settings{Prefix: "APP", Strict: true},
			changeSet{flags.Prefix.Name: true, flags.Strict.Name: true},
		)
		if got.Prefix != "APP" || !got.Strict {
			t.Errorf("options not applied: %+v", got)
		}
	})
}
