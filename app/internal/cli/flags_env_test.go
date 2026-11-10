package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// parseCommand returns a command whose flags are registered by bind and parsed
// from args, with its error stream captured.
func parseCommand(
	t *testing.T, bind func(*pflag.FlagSet), args ...string,
) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	stderr := new(bytes.Buffer)
	cmd.SetErr(stderr)
	bind(cmd.Flags())
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return cmd, stderr
}

func TestBindEnvFlagsOmit(t *testing.T) {
	t.Parallel()

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	bindEnvFlags(fs, envFlag.Name)

	if fs.Lookup(envFlag.Name) != nil {
		t.Error("omitted --env was registered")
	}
	if fs.Lookup(prefixFlag.Name) == nil {
		t.Error("--prefix was not registered")
	}
}

func TestEnvOverridesMapsEveryFlag(t *testing.T) {
	t.Parallel()

	cmd, _ := parseCommand(
		t, func(fs *pflag.FlagSet) { bindEnvFlags(fs) },
		"--env", "prod",
		"--prefix", "PRE_",
		"--suffix", "_SUF",
		"--delimiter", ";",
		"--reference-pattern", `\$\{(\w+)\}`,
		"--overload",
		"--require-overlays=false",
	)
	got := envOverrides(cmd)

	texts := map[string]struct {
		// got is the extracted setting.
		got *string
		// want is the flag value passed on the command line.
		want string
	}{
		"Env":              {got.Env, "prod"},
		"Prefix":           {got.Prefix, "PRE_"},
		"Suffix":           {got.Suffix, "_SUF"},
		"Delimiter":        {got.Delimiter, ";"},
		"ReferencePattern": {got.ReferencePattern, `\$\{(\w+)\}`},
	}
	for field, tc := range texts {
		if tc.got == nil || *tc.got != tc.want {
			t.Errorf("%s = %v, want %q", field, tc.got, tc.want)
		}
	}
	if got.Overload == nil || !*got.Overload {
		t.Errorf("Overload = %v, want true", got.Overload)
	}
	if got.RequireOverlays == nil || *got.RequireOverlays {
		t.Errorf("RequireOverlays = %v, want explicit false", got.RequireOverlays)
	}
}
