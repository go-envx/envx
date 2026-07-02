package envmerge

import (
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestDeepMerge verifies recursive map merging with scalar/list replacement.
func TestDeepMerge(t *testing.T) {
	t.Parallel()

	dst := map[string]any{
		"a": "1",
		"nested": map[string]any{
			"x": "base",
			"y": "keep",
		},
	}
	src := map[string]any{
		"b": "2",
		"nested": map[string]any{
			"x": "override",
		},
	}

	got := deepMerge(dst, src)
	if got["a"] != "1" || got["b"] != "2" {
		t.Errorf("top-level keys wrong: %v", got)
	}
	nested, _ := toMap(got["nested"])
	if nested["x"] != "override" {
		t.Errorf("nested.x = %v, want override", nested["x"])
	}
	if nested["y"] != "keep" {
		t.Errorf("nested.y = %v, want keep", nested["y"])
	}
}

// -------------------------------------------------------------------------------------

// TestFlatten verifies nested-to-env flattening and key normalization.
func TestFlatten(t *testing.T) {
	t.Parallel()

	in := map[string]any{
		"host":     "localhost",
		"password": nil,
		"credentials": map[string]any{
			"user-name": "postgres",
		},
	}
	got, err := flatten(in)
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	if got["HOST"] != "localhost" {
		t.Errorf("HOST = %q, want localhost", got["HOST"])
	}
	if got["CREDENTIALS_USER_NAME"] != "postgres" {
		t.Errorf("CREDENTIALS_USER_NAME = %q, want postgres", got["CREDENTIALS_USER_NAME"])
	}
	if _, ok := got["PASSWORD"]; !ok || got["PASSWORD"] != "" {
		t.Errorf("PASSWORD = %q (present=%t), want empty string for a nil leaf",
			got["PASSWORD"], ok)
	}
}

// -------------------------------------------------------------------------------------

// TestFlattenCollision verifies two paths collapsing to the same key error out.
func TestFlattenCollision(t *testing.T) {
	t.Parallel()

	in := map[string]any{
		"api_key": "a",
		"api": map[string]any{
			"key": "b",
		},
	}
	_, err := flatten(in)
	if err == nil {
		t.Fatal("expected flatten collision error")
	}
	// The colliding paths are reported in a stable, sorted order so the message
	// does not vary with map iteration order.
	if !strings.Contains(err.Error(), `"api.key" and "api_key"`) {
		t.Errorf("collision message not in stable order: %v", err)
	}
}

// -------------------------------------------------------------------------------------

// TestToEnvKey verifies dotted/hyphenated paths normalize to upper-snake.
func TestToEnvKey(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"postgres.user-name": "POSTGRES_USER_NAME",
		"host":               "HOST",
		"a.b.c":              "A_B_C",
	}
	for in, want := range cases {
		if got := toEnvKey(in); got != want {
			t.Errorf("toEnvKey(%q) = %q, want %q", in, got, want)
		}
	}
}
