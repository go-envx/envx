package set

import (
	"reflect"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestApplyFlatKey verifies a flat key is set on a fresh document.
func TestApplyFlatKey(t *testing.T) {
	t.Parallel()

	got := apply(nil, actionParams{Key: "host", Value: "localhost"})
	want := map[string]any{"host": "localhost"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("apply() = %v, want %v", got, want)
	}
}

// -------------------------------------------------------------------------------------

// TestApplyNestedKey verifies dotted keys create intermediate maps.
func TestApplyNestedKey(t *testing.T) {
	t.Parallel()

	got := apply(
		map[string]any{"host": "localhost"},
		actionParams{Key: "credentials.password", Value: "secret"},
	)
	creds, ok := got["credentials"].(map[string]any)
	if !ok {
		t.Fatalf("credentials is not a map: %T", got["credentials"])
	}
	if creds["password"] != "secret" {
		t.Errorf("password = %v, want secret", creds["password"])
	}
	if got["host"] != "localhost" {
		t.Errorf("host = %v, want localhost", got["host"])
	}
}

// -------------------------------------------------------------------------------------

// TestSetNestedKeyOverwritesScalar verifies a scalar blocking a nested path is
// replaced with a map.
func TestSetNestedKeyOverwritesScalar(t *testing.T) {
	t.Parallel()

	data := map[string]any{"a": "scalar"}
	setNestedKey(data, "a.b", "value")
	sub, ok := data["a"].(map[string]any)
	if !ok {
		t.Fatalf("a is not a map: %T", data["a"])
	}
	if sub["b"] != "value" {
		t.Errorf("a.b = %v, want value", sub["b"])
	}
}
