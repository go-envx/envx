package flags

import (
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/schema"
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
	BindBool(cmd, &strict, &schema.Strict)
	BindString(cmd, &prefix, &schema.Prefix)
	BindString(cmd, &suffix, &schema.Suffix)
	BindBool(cmd, &nsPrefix, &schema.NamespacePrefix)

	for _, name := range []string{
		schema.Strict.Name, schema.Prefix.Name, schema.Suffix.Name,
		schema.NamespacePrefix.Name,
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
	BindString(cmd, &env, &schema.Env)

	if cmd.Flags().Lookup(schema.Env.Name) == nil {
		t.Fatalf("flag %q was not registered", schema.Env.Name)
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

// TestBindEnvDefault verifies --env advertises no static default: with nothing set
// the bound value stays empty, leaving the terminal fallback (the first declared
// environment) to resolution.
func TestBindEnvDefault(t *testing.T) {
	t.Parallel()

	var env string
	cmd := newTestCmd()
	BindString(cmd, &env, &schema.Env)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if env != "" {
		t.Errorf("env default = %q, want empty", env)
	}
}

// -------------------------------------------------------------------------------------

// TestBindPersistentString verifies BindPersistentString binds a persistent flag
// so it applies to subcommands.
func TestBindPersistentString(t *testing.T) {
	t.Parallel()

	var path string
	cmd := newTestCmd()
	BindPersistentString(cmd, &path, &schema.Config)

	if cmd.PersistentFlags().Lookup(schema.Config.Name) == nil {
		t.Fatalf("flag %q was not registered as persistent", schema.Config.Name)
	}
}
