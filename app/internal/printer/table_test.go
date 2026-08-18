package printer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/style"
)

// TestWriteTableAlignsPlain verifies column alignment with color disabled,
// including no trailing whitespace on the final column.
func TestWriteTableAlignsPlain(t *testing.T) {
	t.Parallel()

	var out, errStream bytes.Buffer
	p := newTestPrinter(&out, &errStream, false)
	table := Table{
		Headers: []string{"KEY", "STATUS"},
		Rows: [][]Cell{
			{{Text: "API_KEY"}, {Text: "OK", Severity: style.SeverityOK}},
			{{Text: "BAD"}, {Text: "ERROR", Severity: style.SeverityError}},
		},
	}
	if err := p.WriteTable(table); err != nil {
		t.Fatalf("WriteTable() error = %v", err)
	}

	want := strings.Join([]string{
		"KEY      STATUS",
		"API_KEY  OK",
		"BAD      ERROR",
		"",
	}, "\n")
	if got := out.String(); got != want {
		t.Errorf("table =\n%q\nwant\n%q", got, want)
	}
}

// TestWriteTableStylesWhenEnabled verifies the header is bold and severity cells
// are colored, while alignment padding stays outside the escape codes.
func TestWriteTableStylesWhenEnabled(t *testing.T) {
	t.Parallel()

	var out, errStream bytes.Buffer
	p := newTestPrinter(&out, &errStream, true)
	table := Table{
		Headers: []string{"KEY", "STATUS"},
		Rows: [][]Cell{
			{{Text: "API_KEY"}, {Text: "OK", Severity: style.SeverityOK}},
		},
	}
	if err := p.WriteTable(table); err != nil {
		t.Fatalf("WriteTable() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "\033[1mKEY\033[0m") {
		t.Errorf("table %q missing bold header", got)
	}
	if !strings.Contains(got, "\033[36mOK\033[0m") {
		t.Errorf("table %q missing cyan status", got)
	}
	// The gutter padding follows the reset, keeping alignment code-free.
	if !strings.Contains(got, "\033[0m  ") {
		t.Errorf("table %q padded inside the escape codes", got)
	}
}

// TestWriteTableRaggedRow verifies rows shorter than the header render without
// error and pad correctly.
func TestWriteTableRaggedRow(t *testing.T) {
	t.Parallel()

	var out, errStream bytes.Buffer
	p := newTestPrinter(&out, &errStream, false)
	table := Table{
		Headers: []string{"A", "B", "C"},
		Rows: [][]Cell{
			{{Text: "1"}, {Text: "2"}},
		},
	}
	if err := p.WriteTable(table); err != nil {
		t.Fatalf("WriteTable() error = %v", err)
	}

	want := strings.Join([]string{
		"A  B  C",
		"1  2",
		"",
	}, "\n")
	if got := out.String(); got != want {
		t.Errorf("table =\n%q\nwant\n%q", got, want)
	}
}

// TestWriteTableUnicodeWidth verifies column widths are measured by rune count so
// multibyte content aligns correctly.
func TestWriteTableUnicodeWidth(t *testing.T) {
	t.Parallel()

	var out, errStream bytes.Buffer
	p := newTestPrinter(&out, &errStream, false)
	table := Table{
		Headers: []string{"NAME", "X"},
		Rows: [][]Cell{
			{{Text: "café"}, {Text: "1"}},
			{{Text: "naïve"}, {Text: "2"}},
		},
	}
	if err := p.WriteTable(table); err != nil {
		t.Fatalf("WriteTable() error = %v", err)
	}

	want := strings.Join([]string{
		"NAME   X",
		"café   1",
		"naïve  2",
		"",
	}, "\n")
	if got := out.String(); got != want {
		t.Errorf("table =\n%q\nwant\n%q", got, want)
	}
}
