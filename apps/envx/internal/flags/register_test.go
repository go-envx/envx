package flags

import (
	"testing"

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
// TestEngineSettingFlags verifies the individual engine-setting constructors
// bind onto a command and that parsing writes through to their destinations.
func TestEngineSettingFlags(t *testing.T) {
	t.Parallel()

	var (
		strict, nsPrefix bool
		prefix, suffix   string
	)
	cmd := newTestCmd()
	NewStrictFlag(cmd, &strict)
	NewPrefixFlag(cmd, &prefix)
	NewSuffixFlag(cmd, &suffix)
	NewNamespacePrefixFlag(cmd, &nsPrefix)

	for _, name := range []string{
		Strict.Name, Prefix.Name, Suffix.Name, NamespacePrefix.Name,
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
// TestNewEnvFlag verifies the --env flag is bound onto a command and that parsing
// writes through to the destination.
func TestNewEnvFlag(t *testing.T) {
	t.Parallel()

	var env string
	cmd := newTestCmd()
	NewEnvFlag(cmd, &env)

	if cmd.Flags().Lookup(Env.Name) == nil {
		t.Fatalf("flag %q was not registered", Env.Name)
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
// TestNewConfigFlag verifies the --config flag is bound as a persistent flag so
// it applies to subcommands.
func TestNewConfigFlag(t *testing.T) {
	t.Parallel()

	var path string
	cmd := newTestCmd()
	NewConfigFlag(cmd, &path)

	if cmd.PersistentFlags().Lookup(Config.Name) == nil {
		t.Fatalf("flag %q was not registered as persistent", Config.Name)
	}
}

// -------------------------------------------------------------------------------------
// TestNewOutputFlagDefault verifies the --output flag carries its "table"
// default straight from the Output spec when the user sets nothing.
func TestNewOutputFlagDefault(t *testing.T) {
	t.Parallel()

	var out string
	cmd := newTestCmd()
	NewOutputFlag(cmd, &out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != "table" {
		t.Errorf("output default = %q, want table", out)
	}
}
