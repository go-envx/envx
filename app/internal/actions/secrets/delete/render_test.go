package delete

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

// TestRenderReportsRemovedIdentity verifies the summary names the removed secret
// and its store location without any secret value.
func TestRenderReportsRemovedIdentity(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&out, &errOut),
		Result: actionResult{
			Group:     "production",
			Key:       "database_password",
			StorePath: "/workspace/secrets.yaml",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"database_password"`) ||
		!strings.Contains(got, `"production"`) {
		t.Errorf("render() = %q, want the removed identity", got)
	}
	if !strings.Contains(got, "/workspace/secrets.yaml") {
		t.Errorf("render() = %q, want the store location", got)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

// TestRenderStorePathQuotesSpaces verifies a path with a space is quoted so its
// boundary is unambiguous.
func TestRenderStorePathQuotesSpaces(t *testing.T) {
	t.Parallel()

	if got := renderStorePath("/no/space"); got != "/no/space" {
		t.Errorf("renderStorePath() = %q, want the raw path", got)
	}
	const spaced = "/with space/secrets.yaml"
	if got := renderStorePath(spaced); got != `"`+spaced+`"` {
		t.Errorf("renderStorePath() = %q, want a quoted path", got)
	}
}
