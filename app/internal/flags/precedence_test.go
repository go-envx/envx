package flags_test

import (
	"os"
	"testing"

	"github.com/go-envx/envx/app/internal/flags"
)

// strPtr returns a pointer to the string s.
func strPtr(s string) *string {
	return &s
}

// boolPtr returns a pointer to the bool b.
func boolPtr(b bool) *bool {
	return &b
}

// unsetEnv removes key for the duration of the test, restoring the original value
// when the test finishes. It lets a precedence test exercise the "env var absent"
// branch deterministically.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
}

// TestPrecedenceString verifies the string precedence chain: explicit input wins,
// then the ENVX_* var, then the first non-empty layer.
func TestPrecedenceString(t *testing.T) {
	spec := flags.Prefix

	t.Run("explicit wins", func(t *testing.T) {
		t.Setenv(spec.Env, "from-env")
		got := flags.PrecedenceString(&spec, strPtr("from-flag"), strPtr("layer"))
		if got != "from-flag" {
			t.Errorf("got %q, want from-flag", got)
		}
	})
	t.Run("env wins over layers", func(t *testing.T) {
		t.Setenv(spec.Env, "from-env")
		got := flags.PrecedenceString(&spec, nil, strPtr("layer"))
		if got != "from-env" {
			t.Errorf("got %q, want from-env", got)
		}
	})
	t.Run("first non-empty layer", func(t *testing.T) {
		unsetEnv(t, spec.Env)
		got := flags.PrecedenceString(&spec, nil, strPtr(""), strPtr("layer2"))
		if got != "layer2" {
			t.Errorf("got %q, want layer2", got)
		}
	})
}

// TestPrecedenceBool verifies the boolean precedence chain including pointer
// layers.
func TestPrecedenceBool(t *testing.T) {
	spec := flags.RequireOverlays

	t.Run("explicit wins", func(t *testing.T) {
		t.Setenv(spec.Env, "false")
		if !flags.PrecedenceBool(&spec, boolPtr(true), boolPtr(true)) {
			t.Error("expected explicit value true to win")
		}
	})
	t.Run("env parsed", func(t *testing.T) {
		t.Setenv(spec.Env, "true")
		if !flags.PrecedenceBool(&spec, nil) {
			t.Error("expected env value true")
		}
	})
	t.Run("layer pointer", func(t *testing.T) {
		unsetEnv(t, spec.Env)
		if !flags.PrecedenceBool(&spec, nil, nil, boolPtr(true)) {
			t.Error("expected first non-nil layer true")
		}
	})
	t.Run("default false", func(t *testing.T) {
		unsetEnv(t, spec.Env)
		if flags.PrecedenceBool(&spec, nil, nil) {
			t.Error("expected default false")
		}
	})
}
