package cli

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestBindEnvFlagsOmit(t *testing.T) {
	t.Parallel()

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	bindEnvFlags(fs, envFlag.name)

	if fs.Lookup(envFlag.name) != nil {
		t.Error("omitted --env was registered")
	}
	if fs.Lookup(prefixFlag.name) == nil {
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
