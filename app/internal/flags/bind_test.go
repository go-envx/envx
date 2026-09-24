package flags

import (
	"testing"

	"github.com/spf13/pflag"
)

// newFlags returns an empty flag set suitable for registering onto in tests.
func newFlags() *pflag.FlagSet {
	return pflag.NewFlagSet("test", pflag.ContinueOnError)
}

// TestBindString verifies BindString binds a string flag and writes the parsed
// value into the destination.
func TestBindString(t *testing.T) {
	t.Parallel()

	var output string
	fs := newFlags()
	BindString(fs, &output, &Output)
	if err := fs.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if output != "json" {
		t.Errorf("output = %q, want json", output)
	}
}

// TestBindBool verifies BindBool binds a bool flag and writes the parsed value
// into the destination.
func TestBindBool(t *testing.T) {
	t.Parallel()

	var requireOverlays bool
	fs := newFlags()
	BindBool(fs, &requireOverlays, &RequireOverlays)
	if err := fs.Parse([]string{"--require-overlays"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !requireOverlays {
		t.Error("require-overlays = false, want true")
	}
}
