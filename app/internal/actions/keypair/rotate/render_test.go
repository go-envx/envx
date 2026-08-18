package rotate

import (
	"bytes"
	"strings"
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

// TestRender verifies render writes safe rotation metadata and identities.
func TestRender(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&out, &errOut),
		Result: actionResult{
			Result: secrets.UpdateResult{
				Keypairs: []secrets.KeypairMetadata{{
					Group:     "production",
					PublicKey: "public-key",
				}},
				Secrets: []secrets.SecretReference{
					{Group: "production", Key: "api_key"},
				},
			},
			SecretsPath: "secrets.yaml",
			KeysPath:    "envx.keys",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	want := "Rotated keypair for group \"production\":\n" +
		"  public key: public-key\n" +
		"  secrets store: secrets.yaml\n" +
		"  private key file: envx.keys\n" +
		"  re-encrypted 1 secret\n" +
		"    production/api_key\n"
	if got := out.String(); got != want {
		t.Errorf("render = %q, want %q", got, want)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

// TestRenderRejectsEmptyResult verifies render fails when no keypair changed.
func TestRenderRejectsEmptyResult(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := render(&renderParams{
		Printer: plainPrinter(&out, &errOut),
		Result:  actionResult{},
	})
	if err == nil {
		t.Fatal("render() succeeded without a rotated keypair")
	}
	if !strings.Contains(err.Error(), "no keypair change") {
		t.Errorf("error = %q, want no-keypair-change guidance", err)
	}
}
