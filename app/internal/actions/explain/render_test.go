package explain

import (
	"bytes"
	"strings"
	"testing"
)

// sampleResult is a single explain row used by the render tests.
func sampleResult() actionResult {
	return actionResult{Entries: []actionResultEntry{
		{
			Key:       "HOST",
			Value:     "db.local",
			Source:    "/repo/env/postgres.development.yaml",
			SourceKey: "host",
			Shadowed:  []string{"/repo/env/postgres.yaml"},
		},
	}}
}

// TestRenderJSON verifies the JSON format emits a tagged entry array with the
// entry's value.
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

	want := `[
  {
    "key": "HOST",
    "value": "db.local",
    "source": "/repo/env/postgres.development.yaml",
    "sourceKey": "host",
    "shadowed": [
      "/repo/env/postgres.yaml"
    ]
  }
]
`
	if buf.String() != want {
		t.Errorf("render json =\n%s\nwant\n%s", buf.String(), want)
	}
}

// TestRenderTable verifies the table format prints the KEY/VALUE/SOURCE header
// and a row for each entry.
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
		"KEY", "VALUE", "SOURCE",
		"HOST", "db.local", "postgres.development.yaml",
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
