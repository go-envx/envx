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

// TestRenderReportsStorePath verifies safe metadata uses a dedicated path line.
func TestRenderReportsStorePath(t *testing.T) {
	t.Parallel()

	var output, errOut bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&output, &errOut),
		Result: actionResult{
			Group:     "production",
			Key:       "database_password",
			StorePath: "/workspace/secrets.yaml",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	want := "Stored secret \"database_password\" in group \"production\" at:\n" +
		"/workspace/secrets.yaml\n"
	if output.String() != want {
		t.Errorf("rendered output = %q, want %q", output.String(), want)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

// TestRenderQuotesStorePathWithSpaces verifies paths with spaces remain
// unambiguous on their dedicated output line.
func TestRenderQuotesStorePathWithSpaces(t *testing.T) {
	t.Parallel()

	var output, errOut bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&output, &errOut),
		Result: actionResult{
			Group:     "production",
			Key:       "db_pass4",
			StorePath: "/workspace/my secrets/secrets.yaml",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	want := "Stored secret \"db_pass4\" in group \"production\" at:\n" +
		"\"/workspace/my secrets/secrets.yaml\"\n"
	if output.String() != want {
		t.Errorf("rendered output = %q, want %q", output.String(), want)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}
