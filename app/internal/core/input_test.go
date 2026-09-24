package core_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/workspace"
	"github.com/go-envx/envx/app/internal/utils/cliflags"
	"github.com/spf13/pflag"
)

func newFlags() *pflag.FlagSet {
	return pflag.NewFlagSet("test", pflag.ContinueOnError)
}

// TestGetInputUnregisteredStaysNil verifies an option the flag set never registered
// stays nil even though GetInput probes for it.
func TestGetInputUnregisteredStaysNil(t *testing.T) {
	t.Parallel()

	fs := newFlags()
	env.RegisterFlags(fs, env.WithRequireOverlays, env.WithPrefix, env.WithSuffix)
	if fs.Lookup(env.Env.Name) != nil {
		t.Fatal("--env should not be registered without WithEnv")
	}
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if in := core.GetInput(fs); in.Env != nil {
		t.Errorf("Env = %v, want nil", in.Env)
	}
}

// TestGetInputCapturesEveryOption is the guard that locks the field-to-GetInput
// mirror: registering every flag (including --config) and setting each must surface
// a non-nil value, so a new setting cannot be half-wired.
func TestGetInputCapturesEveryOption(t *testing.T) {
	t.Parallel()

	fs := newFlags()
	cliflags.BindString(fs, new(string), &workspace.ConfigFlag)
	env.RegisterFlags(fs,
		env.WithEnv,
		env.WithRequireOverlays,
		env.WithPrefix,
		env.WithSuffix,
		env.WithDelimiter,
		env.WithOverload,
	)
	args := []string{
		"--config", "envx.yaml", "--env", "prod", "--require-overlays", "--prefix", "P",
		"--suffix", "S", "--delimiter", ",", "--overload",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse: %v", err)
	}

	in := core.GetInput(fs)
	if in.ConfigPath == nil || in.Env == nil || in.RequireOverlays == nil ||
		in.Prefix == nil || in.Suffix == nil || in.Delimiter == nil ||
		in.Overload == nil {
		t.Fatalf("a flag was not captured by GetInput: %+v", in)
	}
}

// TestGetInputConfigPath verifies GetInput reads the --config flag, leaving
// ConfigPath nil when it is unset.
func TestGetInputConfigPath(t *testing.T) {
	t.Parallel()

	t.Run("reads --config", func(t *testing.T) {
		fs := newFlags()
		cliflags.BindString(fs, new(string), &workspace.ConfigFlag)
		if err := fs.Parse([]string{"--config", "envx.yaml"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		in := core.GetInput(fs)
		if in.ConfigPath == nil || *in.ConfigPath != "envx.yaml" {
			t.Errorf("ConfigPath = %v, want envx.yaml", in.ConfigPath)
		}
	})
	t.Run("nil when unset", func(t *testing.T) {
		fs := newFlags()
		cliflags.BindString(fs, new(string), &workspace.ConfigFlag)
		if err := fs.Parse(nil); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if in := core.GetInput(fs); in.ConfigPath != nil {
			t.Errorf("ConfigPath = %v, want nil", in.ConfigPath)
		}
	})
}
