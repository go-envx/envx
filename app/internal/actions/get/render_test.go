package get

import (
	"bytes"
	"testing"
)

// TestRender verifies render writes the resolved value followed by a newline.
func TestRender(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := render(&renderParams{
		Writer: &buf,
		Result: actionResult{Value: "dev-db.local", Source: "postgres.yaml"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := buf.String(); got != "dev-db.local\n" {
		t.Errorf("render = %q, want %q", got, "dev-db.local\n")
	}
}

// TestRenderEmptyValue verifies an empty value still emits a trailing newline.
func TestRenderEmptyValue(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := render(&renderParams{Writer: &buf, Result: actionResult{}})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := buf.String(); got != "\n" {
		t.Errorf("render = %q, want newline", got)
	}
}
