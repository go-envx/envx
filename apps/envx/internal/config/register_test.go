package config

import (
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/settings"
	"github.com/spf13/cobra"
)

// -------------------------------------------------------------------------------------
// newTestCmd returns a no-op command suitable for binding flags onto in tests.
func newTestCmd() *cobra.Command {
	return &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error {
		return nil
	}}
}

// -------------------------------------------------------------------------------------
// TestBindEngineSettingFlags verifies the generic binders register the engine-
// setting specs onto a command and that parsing writes through to their
// destinations.
func TestBindEngineSettingFlags(t *testing.T) {
	t.Parallel()

	var (
		strict, nsPrefix bool
		prefix, suffix   string
	)
	cmd := newTestCmd()
	BindBool(cmd, &strict, &settings.Strict)
	BindString(cmd, &prefix, &settings.Prefix)
	BindString(cmd, &suffix, &settings.Suffix)
	BindBool(cmd, &nsPrefix, &settings.NamespacePrefix)

	for _, name := range []string{
		settings.Strict.Name, settings.Prefix.Name, settings.Suffix.Name,
		settings.NamespacePrefix.Name,
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

	if !strict || !nsPrefix {
		t.Errorf("bool flags not applied: strict=%v nsPrefix=%v", strict, nsPrefix)
	}
	if prefix != "APP" || suffix != "V2" {
		t.Errorf("string flags not applied: prefix=%q suffix=%q", prefix, suffix)
	}
}

// -------------------------------------------------------------------------------------
// TestBindString verifies BindString binds a string flag onto a command and that
// parsing writes through to the destination.
func TestBindString(t *testing.T) {
	t.Parallel()

	var env string
	cmd := newTestCmd()
	BindString(cmd, &env, &settings.Env)

	if cmd.Flags().Lookup(settings.Env.Name) == nil {
		t.Fatalf("flag %q was not registered", settings.Env.Name)
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
// TestBindPersistentString verifies BindPersistentString binds a persistent flag
// so it applies to subcommands.
func TestBindPersistentString(t *testing.T) {
	t.Parallel()

	var path string
	cmd := newTestCmd()
	BindPersistentString(cmd, &path, &settings.Config)

	if cmd.PersistentFlags().Lookup(settings.Config.Name) == nil {
		t.Fatalf("flag %q was not registered as persistent", settings.Config.Name)
	}
}

// -------------------------------------------------------------------------------------
// TestBindStringDefault verifies BindString carries a spec's default (Output's
// "table") through to the destination when the user sets nothing.
func TestBindStringDefault(t *testing.T) {
	t.Parallel()

	var out string
	cmd := newTestCmd()
	BindString(cmd, &out, &settings.Output)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != "table" {
		t.Errorf("output default = %q, want table", out)
	}
}
