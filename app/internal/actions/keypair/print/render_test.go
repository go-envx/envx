package print

import (
	"bytes"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/printer"
)

// plainPrinter builds a printer over the given sinks with color forced off so
// assertions can match exact, unstyled output.
func plainPrinter(out, errOut *bytes.Buffer) *printer.Printer {
	disabled := false
	return printer.New(printer.Options{Out: out, Err: errOut, Color: &disabled})
}

// TestRender verifies render writes self-identifying key values with display
// spacing after their type prefixes.
func TestRender(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&out, &errOut),
		Result: actionResult{
			Keypair: cipher.Keypair{
				PublicKey:  "age-public-key:age1public",
				PrivateKey: "age-private-key:AGE-SECRET-KEY-1PRIVATE",
			},
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	want := "age-public-key: age1public\n" +
		"age-private-key: AGE-SECRET-KEY-1PRIVATE\n"
	if got := out.String(); got != want {
		t.Errorf("render = %q, want %q", got, want)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}
