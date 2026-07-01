package flags

import (
	"testing"

	"github.com/go-envx/envx/app/internal/schema"
)

// -------------------------------------------------------------------------------------

// TestBindString verifies BindString binds a string flag and writes the parsed
// value into the destination.
func TestBindString(t *testing.T) {
	t.Parallel()

	var output string
	fs := newFlags()
	BindString(fs, &output, &schema.Output)
	if err := fs.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if output != "json" {
		t.Errorf("output = %q, want json", output)
	}
}

// -------------------------------------------------------------------------------------

// TestBindBool verifies BindBool binds a bool flag and writes the parsed value
// into the destination.
func TestBindBool(t *testing.T) {
	t.Parallel()

	var reveal bool
	fs := newFlags()
	BindBool(fs, &reveal, &schema.Reveal)
	if err := fs.Parse([]string{"--reveal"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !reveal {
		t.Error("reveal = false, want true")
	}
}
