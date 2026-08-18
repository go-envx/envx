package diff

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/printer"
)

// plainPrinter builds a printer over the given sinks with color forced off so
// assertions can match exact, unstyled output.
func plainPrinter(out, errOut *bytes.Buffer) *printer.Printer {
	disabled := false
	return printer.New(printer.Options{Out: out, Err: errOut, Color: &disabled})
}

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

	var buf, errBuf bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&buf, &errBuf),
		Result:  sampleResult(),
		Format:  "json",
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

	var buf, errBuf bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&buf, &errBuf),
		Result:  sampleResult(),
		Format:  "table",
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

// TestRenderTableColored verifies the table applies the conventional diff
// palette when color is enabled: green additions, red removals, yellow changes.
func TestRenderTableColored(t *testing.T) {
	t.Parallel()

	enabled := true
	var buf, errBuf bytes.Buffer
	pr := printer.New(printer.Options{Out: &buf, Err: &errBuf, Color: &enabled})
	err := render(&renderParams{
		Printer: pr,
		Result:  sampleResult(),
		Format:  "table",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"\033[32m+\033[0m", // green addition
		"\033[31m-\033[0m", // red removal
		"\033[33m~\033[0m", // yellow change
	} {
		if !strings.Contains(out, want) {
			t.Errorf("colored table missing %q:\n%q", want, out)
		}
	}
}

// TestRenderTableNoDiff verifies an empty diff prints nothing at all.
func TestRenderTableNoDiff(t *testing.T) {
	t.Parallel()

	var buf, errBuf bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&buf, &errBuf),
		Result:  actionResult{},
		Format:  "table",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("no-diff table wrote %q, want nothing", buf.String())
	}
}

// TestRenderInvalidFormat verifies an unrecognized output format is rejected
// rather than silently falling back to the table.
func TestRenderInvalidFormat(t *testing.T) {
	t.Parallel()

	var buf, errBuf bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&buf, &errBuf),
		Result:  sampleResult(),
		Format:  "jsonn",
	})
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
}
