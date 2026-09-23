package printer

import (
	"bytes"
	"strings"
	"testing"
)

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
