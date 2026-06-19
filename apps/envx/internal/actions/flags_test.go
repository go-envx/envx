package actions

import (
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/spf13/cobra"
)

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
