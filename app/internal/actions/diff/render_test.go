package diff

import (
	"bytes"
	"strings"
	"testing"
)

// sampleResult is a small diff covering all three change kinds, used by the
// render tests.
func sampleResult() actionResult {
	return actionResult{
		Added:   []actionResultChange{{Key: "NEW", EnvB: "added-value"}},
		Removed: []actionResultChange{{Key: "OLD", EnvA: "removed-value"}},
		Changed: []actionResultChange{{Key: "MOD", EnvA: "before", EnvB: "after"}},
	}
}

// TestRenderJSON verifies the JSON format emits added, removed, and changed
// sections with their values.
func TestRenderJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := render(&renderParams{
		Writer: &buf,
		Result: sampleResult(),
		Format: "json",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	want := `{
  "added": [
    {
      "key": "NEW",
      "env_b": "added-value"
    }
  ],
  "removed": [
    {
      "key": "OLD",
      "env_a": "removed-value"
    }
  ],
  "changed": [
    {
      "key": "MOD",
      "env_a": "before",
      "env_b": "after"
    }
  ]
}
`
	if buf.String() != want {
		t.Errorf("render json =\n%s\nwant\n%s", buf.String(), want)
	}
}

// TestRenderTable verifies the table format prints sign-prefixed rows with the
// arrow notation for changed keys.
func TestRenderTable(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := render(&renderParams{
		Writer: &buf,
		Result: sampleResult(),
		Format: "table",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"+", "NEW", "added-value",
		"-", "OLD", "removed-value",
		"~", "MOD", "before -> after",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q:\n%s", want, out)
		}
	}
}

// TestRenderInvalidFormat verifies an unrecognized output format is rejected
// rather than silently falling back to the table.
func TestRenderInvalidFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := render(&renderParams{
		Writer: &buf,
		Result: sampleResult(),
		Format: "jsonn",
	})
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
}
