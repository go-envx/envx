package flags

import (
	"testing"

	"github.com/spf13/pflag"
)

// -------------------------------------------------------------------------------------

// newFlags returns an empty flag set suitable for registering onto in tests.
func newFlags() *pflag.FlagSet {
	return pflag.NewFlagSet("test", pflag.ContinueOnError)
}

// -------------------------------------------------------------------------------------

// TestRegisterAndGetInput verifies Register + GetInput round-trip the explicitly-set
// option flags into a *config.Input, leaving unset and unregistered flags nil.
func TestRegisterAndGetInput(t *testing.T) {
	t.Parallel()

	fs := newFlags()
	Register(fs, WithEnv, WithRequireOverlays, WithPrefix, WithSuffix, WithNamespacePrefix)
	args := []string{"--env", "production", "--prefix", "APP", "--require-overlays"}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse: %v", err)
	}

	in := GetInput(fs)
	if in.Env == nil || *in.Env != "production" {
		t.Errorf("Env = %v, want production", in.Env)
	}
	if in.Prefix == nil || *in.Prefix != "APP" {
		t.Errorf("Prefix = %v, want APP", in.Prefix)
	}
	if in.RequireOverlays == nil || !*in.RequireOverlays {
		t.Errorf("RequireOverlays = %v, want true", in.RequireOverlays)
	}
	if in.Suffix != nil {
		t.Errorf("Suffix = %v, want nil (unset)", in.Suffix)
	}
	if in.Overload != nil {
		t.Errorf("Overload = %v, want nil (unregistered)", in.Overload)
	}
}
