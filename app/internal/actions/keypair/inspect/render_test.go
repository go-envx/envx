package inspect

import (
	"bytes"
	"testing"

	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/internal/secrets"
)

// plainPrinter builds a printer over the given sinks with color forced off so
// assertions can match exact, unstyled output.
func plainPrinter(out, errOut *bytes.Buffer) *printer.Printer {
	disabled := false
	return printer.New(printer.Options{Out: out, Err: errOut, Color: &disabled})
}

// TestRender verifies render writes public metadata and private-key status.
func TestRender(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&out, &errOut),
		Result: secrets.KeypairMetadata{
			Group:            "production",
			PublicKey:        "public-key",
			PrivateKeyStatus: "valid",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	want := "Keypair for group \"production\":\n" +
		"  public key: public-key\n" +
		"  private key status: valid\n"
	if got := out.String(); got != want {
		t.Errorf("render = %q, want %q", got, want)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}
