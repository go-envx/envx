package printer

import (
	"bytes"
	"strings"
	"testing"
)

// boolPtr returns a pointer to b for setting Options.Color explicitly.
func boolPtr(b bool) *bool {
	return &b
}

// newTestPrinter builds a Printer writing to the provided buffers with color
// forced to the given state, so output is deterministic regardless of terminal.
func newTestPrinter(out, errStream *bytes.Buffer, color bool) *Printer {
	return New(Options{Out: out, Err: errStream, Color: boolPtr(color)})
}

// TestLogMessage verifies a plain line is written to standard output, uncolored.
func TestLogMessage(t *testing.T) {
	t.Parallel()

	var out, errStream bytes.Buffer
	p := newTestPrinter(&out, &errStream, true)
	if err := p.LogMessage("done"); err != nil {
		t.Fatalf("LogMessage() error = %v", err)
	}

	if got := out.String(); got != "done\n" {
		t.Errorf("out = %q, want %q", got, "done\n")
	}
	if errStream.Len() != 0 {
		t.Errorf("err = %q, want empty", errStream.String())
	}
}

// TestLogWarning verifies warnings carry the glyph, go to standard error, and
// are colored only when color is enabled.
func TestLogWarning(t *testing.T) {
	t.Parallel()

	var out, errStream bytes.Buffer
	p := newTestPrinter(&out, &errStream, true)
	if err := p.LogWarning("missing key"); err != nil {
		t.Fatalf("LogWarning() error = %v", err)
	}

	got := errStream.String()
	if !strings.Contains(got, warningGlyph+"  "+warningLabel) {
		t.Errorf("warning %q missing glyph and label", got)
	}
	if !strings.Contains(got, "\033[33m") {
		t.Errorf("warning %q missing yellow code", got)
	}
	if out.Len() != 0 {
		t.Errorf("out = %q, want empty", out.String())
	}
}

// TestLogWarningNoColor verifies warnings omit escape codes and the glyph when
// color is off, relying on the label to carry severity.
func TestLogWarningNoColor(t *testing.T) {
	t.Parallel()

	var out, errStream bytes.Buffer
	p := newTestPrinter(&out, &errStream, false)
	if err := p.LogWarning("missing key"); err != nil {
		t.Fatalf("LogWarning() error = %v", err)
	}

	want := warningLabel + " missing key\n"
	if got := errStream.String(); got != want {
		t.Errorf("warning = %q, want %q", got, want)
	}
	if strings.Contains(errStream.String(), warningGlyph) {
		t.Error("no-color warning included the glyph")
	}
}

// TestLogErrorNoColor verifies errors omit escape codes and the glyph when color
// is off, relying on the label to carry severity.
func TestLogErrorNoColor(t *testing.T) {
	t.Parallel()

	var out, errStream bytes.Buffer
	p := newTestPrinter(&out, &errStream, false)
	if err := p.LogError("3 value(s) failed"); err != nil {
		t.Fatalf("LogError() error = %v", err)
	}

	want := errorLabel + " 3 value(s) failed\n"
	if got := errStream.String(); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
	if strings.Contains(errStream.String(), errorGlyph) {
		t.Error("no-color error included the glyph")
	}
}

// TestLogError verifies errors carry the glyph, go to standard error, and are
// bold-red when color is enabled.
func TestLogError(t *testing.T) {
	t.Parallel()

	var out, errStream bytes.Buffer
	p := newTestPrinter(&out, &errStream, true)
	if err := p.LogError("3 value(s) failed"); err != nil {
		t.Fatalf("LogError() error = %v", err)
	}

	got := errStream.String()
	if !strings.Contains(got, errorGlyph+"  "+errorLabel) {
		t.Errorf("error %q missing glyph and label", got)
	}
	if !strings.Contains(got, "\033[1m") || !strings.Contains(got, "\033[31m") {
		t.Errorf("error %q missing bold-red codes", got)
	}
	if out.Len() != 0 {
		t.Errorf("out = %q, want empty", out.String())
	}
}

// TestWriteJSON verifies JSON is indented, written to standard output, and never
// colored even with color enabled.
func TestWriteJSON(t *testing.T) {
	t.Parallel()

	var out, errStream bytes.Buffer
	p := newTestPrinter(&out, &errStream, true)
	value := map[string]int{"count": 2}
	if err := p.WriteJSON(value); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	want := "{\n  \"count\": 2\n}\n"
	if got := out.String(); got != want {
		t.Errorf("json = %q, want %q", got, want)
	}
	if strings.Contains(out.String(), "\033") {
		t.Error("json output contained an escape code")
	}
}

// TestNewAutoDetectsNonTerminal verifies a non-terminal writer with no override
// disables color.
func TestNewAutoDetectsNonTerminal(t *testing.T) {
	t.Parallel()

	var out, errStream bytes.Buffer
	p := New(Options{Out: &out, Err: &errStream})
	if p.outStyle.Enabled() {
		t.Error("outStyle enabled for a non-terminal writer")
	}
	if p.errStyle.Enabled() {
		t.Error("errStyle enabled for a non-terminal writer")
	}
}
