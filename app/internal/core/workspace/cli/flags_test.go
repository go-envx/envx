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
	if bind != nil {
		bind(cmd.Flags())
	}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return cmd, stderr
}

func TestBindConfig(t *testing.T) {
	t.Run("default is empty", func(t *testing.T) {
		cmd, _ := parseCommand(t, BindConfig)
		if got := ConfigPath(cmd); got != "" {
			t.Errorf("ConfigPath() = %q, want empty", got)
		}
	})

	t.Run("explicit flag", func(t *testing.T) {
		cmd, _ := parseCommand(t, BindConfig, "--config", "custom/envx.yaml")
		if got := ConfigPath(cmd); got != "custom/envx.yaml" {
			t.Errorf("ConfigPath() = %q, want custom/envx.yaml", got)
		}
	})

	t.Run("env fallback", func(t *testing.T) {
		t.Setenv("ENVX_CONFIG", "from/env/envx.yaml")
		cmd, _ := parseCommand(t, BindConfig)
		if got := ConfigPath(cmd); got != "from/env/envx.yaml" {
			t.Errorf("ConfigPath() = %q, want from/env/envx.yaml", got)
		}
	})

	t.Run("precedence: flag over env", func(t *testing.T) {
		t.Setenv("ENVX_CONFIG", "from/env/envx.yaml")
		cmd, _ := parseCommand(t, BindConfig, "--config", "from/flag/envx.yaml")
		if got := ConfigPath(cmd); got != "from/flag/envx.yaml" {
			t.Errorf("ConfigPath() = %q, want from/flag/envx.yaml", got)
		}
	})

	t.Run("unregistered flag ignores env", func(t *testing.T) {
		t.Setenv("ENVX_CONFIG", "from/env/envx.yaml")
		cmd, _ := parseCommand(t, nil)
		if got := ConfigPath(cmd); got != "" {
			t.Errorf("ConfigPath() = %q, want empty for unregistered flag", got)
		}
	})
}

func TestBindSettingsOmission(t *testing.T) {
	t.Parallel()

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindSettings(fs, "env", "prefix")

	if fs.Lookup("env") != nil {
		t.Error("omitted --env was registered")
	}
	if fs.Lookup("prefix") != nil {
		t.Error("omitted --prefix was registered")
	}
	if fs.Lookup("suffix") == nil {
		t.Error("--suffix was not registered")
	}
	if fs.Lookup("delimiter") == nil {
		t.Error("--delimiter was not registered")
	}
}

func TestSettingsPrecedenceAndFallback(t *testing.T) {
	t.Run("explicit flag overrides env", func(t *testing.T) {
		t.Setenv("ENVX_PREFIX", "ENV_")
		t.Setenv("ENVX_OVERLOAD", "false")

		cmd, _ := parseCommand(
			t, func(fs *pflag.FlagSet) { BindSettings(fs) },
			"--prefix", "FLAG_",
			"--overload",
		)
		got := Settings(cmd)

		if got.Prefix == nil || *got.Prefix != "FLAG_" {
			t.Errorf("Prefix = %v, want FLAG_", got.Prefix)
		}
		if got.Overload == nil || !*got.Overload {
			t.Errorf("Overload = %v, want true", got.Overload)
		}
	})

	t.Run("env fallback when flag unset", func(t *testing.T) {
		t.Setenv("ENVX_ENV", "staging")
		t.Setenv("ENVX_SUFFIX", "_SUF")
		t.Setenv("ENVX_REQUIRE_OVERLAYS", "false")

		cmd, _ := parseCommand(t, func(fs *pflag.FlagSet) { BindSettings(fs) })
		got := Settings(cmd)

		if got.Env == nil || *got.Env != "staging" {
			t.Errorf("Env = %v, want staging", got.Env)
		}
		if got.Suffix == nil || *got.Suffix != "_SUF" {
			t.Errorf("Suffix = %v, want _SUF", got.Suffix)
		}
		if got.RequireOverlays == nil || *got.RequireOverlays {
			t.Errorf("RequireOverlays = %v, want explicit false", got.RequireOverlays)
		}
		if got.Prefix != nil {
			t.Errorf("Prefix = %v, want nil for unset setting", got.Prefix)
		}
	})

	t.Run("unregistered flag yields nil without reading env", func(t *testing.T) {
		t.Setenv("ENVX_ENV", "staging")
		t.Setenv("ENVX_PREFIX", "PRE_")

		// BindSettings with "env" and "prefix" omitted.
		cmd, _ := parseCommand(t, func(fs *pflag.FlagSet) {
			BindSettings(fs, "env", "prefix")
		})
		got := Settings(cmd)

		if got.Env != nil {
			t.Errorf("Env = %v, want nil for unregistered flag", got.Env)
		}
		if got.Prefix != nil {
			t.Errorf("Prefix = %v, want nil for unregistered flag", got.Prefix)
		}
	})
}

func TestSettingsAllFlags(t *testing.T) {
	t.Parallel()

	cmd, _ := parseCommand(
		t, func(fs *pflag.FlagSet) { BindSettings(fs) },
		"--env", "prod",
		"--prefix", "PRE_",
		"--suffix", "_SUF",
		"--delimiter", ";",
		"--reference-pattern", `\$\{(\w+)\}`,
		"--overload",
		"--require-overlays=false",
	)
	got := Settings(cmd)

	texts := map[string]struct {
		got  *string
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
