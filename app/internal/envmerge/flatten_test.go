package envmerge

import (
	"strings"
	"testing"
)

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
	if value, _ := renderLeafValue(got["HOST"], "host", ","); value != "localhost" {
		t.Errorf("HOST = %q, want localhost", value)
	}
	if value, _ := renderLeafValue(
		got["CREDENTIALS_USER_NAME"], "credentials.user-name", ",",
	); value != "postgres" {
		t.Errorf("CREDENTIALS_USER_NAME = %q, want postgres", value)
	}
	password, ok := got["PASSWORD"]
	value, _ := renderLeafValue(password, "password", ",")
	if !ok || value != "" {
		t.Errorf("PASSWORD = %q (present=%t), want empty string for a nil leaf", value, ok)
	}
}

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

// TestFlattenList verifies a list leaf is joined into a single delimiter-
// separated string, with an empty list and a nil item rendering as empty
// segments.
func TestFlattenList(t *testing.T) {
	t.Parallel()

	in := map[string]any{
		"hosts": []any{"a", "b", "c"},
		"ports": []any{5432, 5433},
		"empty": []any{},
		"gappy": []any{"x", nil, "z"},
	}

	got, err := flatten(in)
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	hosts, _ := renderLeafValue(got["HOSTS"], "hosts", ",")
	if hosts != "a,b,c" {
		t.Errorf("HOSTS = %q, want a,b,c", hosts)
	}
	ports, _ := renderLeafValue(got["PORTS"], "ports", ",")
	if ports != "5432,5433" {
		t.Errorf("PORTS = %q, want 5432,5433", ports)
	}
	empty, _ := renderLeafValue(got["EMPTY"], "empty", ",")
	if empty != "" {
		t.Errorf("EMPTY = %q, want empty string", empty)
	}
	gappy, _ := renderLeafValue(got["GAPPY"], "gappy", ",")
	if gappy != "x,,z" {
		t.Errorf("GAPPY = %q, want x,,z", gappy)
	}
}

// TestFlattenListCustomDelimiter verifies the join delimiter is configurable.
func TestFlattenListCustomDelimiter(t *testing.T) {
	t.Parallel()

	in := map[string]any{"path": []any{"/bin", "/usr/bin"}}
	got, err := flatten(in)
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	value, err := renderLeafValue(got["PATH"], "path", ":")
	if err != nil {
		t.Fatalf("renderLeafValue: %v", err)
	}
	if value != "/bin:/usr/bin" {
		t.Errorf("PATH = %q, want /bin:/usr/bin", value)
	}
}

// TestFlattenListErrors verifies a list is rejected when an item contains the
// delimiter (ambiguous to split back) or is itself a non-scalar (no flat form).
func TestFlattenListErrors(t *testing.T) {
	t.Parallel()

	t.Run("item contains delimiter", func(t *testing.T) {
		t.Parallel()
		value := leafValue{items: []string{"a,b", "c"}, list: true}
		if _, err := renderLeafValue(value, "hosts", ","); err == nil {
			t.Fatal("expected error for item containing the delimiter")
		}
	})
	t.Run("non-scalar item", func(t *testing.T) {
		t.Parallel()
		in := map[string]any{"servers": []any{map[string]any{"host": "a"}}}
		if _, err := flatten(in); err == nil {
			t.Fatal("expected error for non-scalar list item")
		}
	})
}

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
