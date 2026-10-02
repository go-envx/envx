package env_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/features/env"
)

// strPtr returns a pointer to the string s.
func strPtr(s string) *string {
	return &s
}

// boolPtr returns a pointer to the bool b.
func boolPtr(b bool) *bool {
	return &b
}

// TestPrecedenceString verifies the string precedence chain: explicit input wins,
// then the first non-empty layer.
func TestPrecedenceString(t *testing.T) {
	t.Run("explicit wins", func(t *testing.T) {
		t.Setenv("ENVX_PREFIX", "from-env")
		got := env.PrecedenceString(strPtr("from-flag"), strPtr("layer"))
		if got != "from-flag" {
			t.Errorf("got %q, want from-flag", got)
		}
	})
	t.Run("ignores process environment", func(t *testing.T) {
		t.Setenv("ENVX_PREFIX", "from-env")
		got := env.PrecedenceString(nil, strPtr("layer"))
		if got != "layer" {
			t.Errorf("got %q, want layer", got)
		}
	})
	t.Run("first non-empty layer", func(t *testing.T) {
		got := env.PrecedenceString(nil, strPtr(""), strPtr("layer2"))
		if got != "layer2" {
			t.Errorf("got %q, want layer2", got)
		}
	})
	t.Run("default empty", func(t *testing.T) {
		if got := env.PrecedenceString(nil, nil, strPtr("")); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

// TestPrecedenceBool verifies the boolean precedence chain including pointer layers.
func TestPrecedenceBool(t *testing.T) {
	t.Run("explicit wins", func(t *testing.T) {
		if env.PrecedenceBool(boolPtr(false), boolPtr(true)) {
			t.Error("expected explicit value false to win")
		}
	})
	t.Run("ignores process environment", func(t *testing.T) {
		t.Setenv("ENVX_REQUIRE_OVERLAYS", "true")
		if env.PrecedenceBool(nil) {
			t.Error("expected process environment to be ignored")
		}
	})
	t.Run("layer pointer", func(t *testing.T) {
		if !env.PrecedenceBool(nil, nil, boolPtr(true)) {
			t.Error("expected first non-nil layer true")
		}
	})
	t.Run("default false", func(t *testing.T) {
		if env.PrecedenceBool(nil, nil) {
			t.Error("expected default false")
		}
	})
}
