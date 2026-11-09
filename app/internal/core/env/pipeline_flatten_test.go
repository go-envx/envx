package env

import (
	"errors"
	"strings"
	"testing"
)

// TestFlatten verifies nested keys normalize to upper-snake names, map back to
// their dotted paths, and leaves render with their scalar or list shape.
func TestFlatten(t *testing.T) {
	t.Parallel()

	got, paths, err := flatten(map[string]any{
		"host":        "localhost",
		"password":    nil,
		"credentials": map[any]any{"user-name": "postgres"},
		"ports":       []any{5432, 5433},
		"empty":       []any{},
		"gappy":       []any{"x", nil, "z"},
	})
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	want := map[string]string{
		"HOST":                  "localhost",
		"PASSWORD":              "",
		"CREDENTIALS_USER_NAME": "postgres",
		"PORTS":                 "5432,5433",
		"EMPTY":                 "",
		"GAPPY":                 "x,,z",
	}
	if len(got) != len(want) {
		t.Fatalf("flatten keys = %d, want %d", len(got), len(want))
	}
	for key, text := range want {
		rendered, err := renderLeaf(got[key], key, ",")
		if err != nil || rendered != text {
			t.Errorf("%s = %q, %v; want %q", key, rendered, err, text)
		}
	}
	if got := paths["CREDENTIALS_USER_NAME"]; got != "credentials.user-name" {
		t.Errorf("CREDENTIALS_USER_NAME path = %q, want credentials.user-name", got)
	}
}

// TestFlattenErrors verifies collisions report paths in stable order and
// non-scalar list items are rejected.
func TestFlattenErrors(t *testing.T) {
	t.Parallel()

	_, _, err := flatten(map[string]any{
		"api_key": "a",
		"api":     map[string]any{"key": "b"},
	})
	if !errors.Is(err, ErrFlattenCollision) ||
		!strings.Contains(err.Error(), `"api.key" and "api_key"`) {
		t.Errorf("collision error = %v", err)
	}

	_, _, err = flatten(map[string]any{"servers": []any{map[string]any{"host": "a"}}})
	if err == nil {
		t.Error("non-scalar list item flattened")
	}
}

// TestRenderLeaf verifies list joining and delimiter rejection without leaking
// the item.
func TestRenderLeaf(t *testing.T) {
	t.Parallel()

	list := leafValue{items: []string{"/bin", "/usr/bin"}, list: true}
	if got, err := renderLeaf(list, "path", ":"); err != nil || got != "/bin:/usr/bin" {
		t.Errorf("renderLeaf = %q, %v; want /bin:/usr/bin", got, err)
	}

	bad := leafValue{items: []string{"se,cret", "c"}, list: true}
	_, err := renderLeaf(bad, "hosts", ",")
	if err == nil || strings.Contains(err.Error(), "se,cret") {
		t.Errorf("renderLeaf error = %v, want a delimiter error without the value", err)
	}
}

// TestToEnvKey verifies dots and hyphens become underscores.
func TestToEnvKey(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"postgres.user-name": "POSTGRES_USER_NAME",
		"host":               "HOST",
		"a.b.c":              "A_B_C",
	} {
		if got := toEnvKey(in); got != want {
			t.Errorf("toEnvKey(%q) = %q, want %q", in, got, want)
		}
	}
}
