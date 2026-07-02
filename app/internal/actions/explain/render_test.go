package explain

import (
	"bytes"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestMaskResult verifies values are redacted unless reveal is set, and that
// empty values stay empty.
func TestMaskResult(t *testing.T) {
	t.Parallel()

	in := actionResult{Entries: []actionResultEntry{
		{Key: "A", Value: "secret"},
		{Key: "B", Value: ""},
	}}

	masked := maskResult(in, false)
	if masked.Entries[0].Value != redacted {
		t.Errorf("entry value = %q, want redacted", masked.Entries[0].Value)
	}
	if masked.Entries[1].Value != "" {
		t.Errorf("empty value changed to %q, want empty", masked.Entries[1].Value)
	}

	revealed := maskResult(in, true)
	if revealed.Entries[0].Value != "secret" {
		t.Errorf("reveal should keep value, got %q", revealed.Entries[0].Value)
	}
}

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

// TestRenderJSON verifies the JSON format emits a tagged entry array with the
// value revealed when reveal is set.
func TestRenderJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := render(&renderParams{
		Writer: &buf,
		Result: sampleResult(),
		Format: "json",
		Reveal: true,
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

// -------------------------------------------------------------------------------------

// TestRenderTable verifies the table format prints the KEY/VALUE/SOURCE header
// and a row for each entry.
func TestRenderTable(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := render(&renderParams{
		Writer: &buf,
		Result: sampleResult(),
		Format: "table",
		Reveal: true,
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

// -------------------------------------------------------------------------------------

// TestRenderMasksByDefault verifies render redacts values when reveal is not set.
func TestRenderMasksByDefault(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := render(&renderParams{
		Writer: &buf,
		Result: sampleResult(),
		Format: "table",
		Reveal: false,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, redacted) {
		t.Errorf("expected redacted placeholder, got:\n%s", out)
	}
	if strings.Contains(out, "db.local") {
		t.Errorf("plaintext value leaked into masked output:\n%s", out)
	}
}

// -------------------------------------------------------------------------------------

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
