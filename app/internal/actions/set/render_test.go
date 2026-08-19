package set

import (
	"bytes"
	"testing"

	"github.com/go-envx/envx/app/internal/printer"
)

// plainPrinter builds a printer over the given sinks with color forced off so
// assertions can match exact, unstyled output.
func plainPrinter(out, errOut *bytes.Buffer) *printer.Printer {
	disabled := false
	return printer.New(printer.Options{Out: out, Err: errOut, Color: &disabled})
}

// TestRenderReportsOverlayPath verifies the confirmation names the key and uses
// a dedicated path line.
func TestRenderReportsOverlayPath(t *testing.T) {
	t.Parallel()

	var output, errOut bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&output, &errOut),
		Result: actionResult{
			Key:         "credentials.password",
			OverlayPath: "/workspace/env/database.yaml",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	want := "Set \"credentials.password\" in:\n" +
		"/workspace/env/database.yaml\n"
	if output.String() != want {
		t.Errorf("rendered output = %q, want %q", output.String(), want)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

// TestRenderQuotesOverlayPathWithSpaces verifies paths with spaces remain
// unambiguous on their dedicated output line.
func TestRenderQuotesOverlayPathWithSpaces(t *testing.T) {
	t.Parallel()

	var output, errOut bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&output, &errOut),
		Result: actionResult{
			Key:         "host",
			OverlayPath: "/workspace/my env/database.yaml",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	want := "Set \"host\" in:\n" +
		"\"/workspace/my env/database.yaml\"\n"
	if output.String() != want {
		t.Errorf("rendered output = %q, want %q", output.String(), want)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}
