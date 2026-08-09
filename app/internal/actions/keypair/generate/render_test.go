package generate

import (
	"bytes"
	"testing"

	"github.com/go-envx/envx/app/internal/secrets"
)

// -------------------------------------------------------------------------------------

// TestRender verifies render writes safe generation metadata and paths.
func TestRender(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := render(&renderParams{
		Writer: &output,
		Result: actionResult{
			Metadata: secrets.KeypairMetadata{
				Group:     "production",
				PublicKey: "public-key",
			},
			SecretsPath: "secrets.yaml",
			KeysPath:    "envx.keys",
		},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	want := "Generated keypair for group \"production\":\n" +
		"  public key: public-key\n" +
		"  secrets store: secrets.yaml\n" +
		"  private key file: envx.keys\n"
	if got := output.String(); got != want {
		t.Errorf("render = %q, want %q", got, want)
	}
}
