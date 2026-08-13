package rotate

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/secrets"
)

// TestRender verifies render writes safe rotation metadata and identities.
func TestRender(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := render(&renderParams{
		Writer: &output,
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
		"  re-encrypted 1 secret(s)\n" +
		"    production/api_key\n"
	if got := output.String(); got != want {
		t.Errorf("render = %q, want %q", got, want)
	}
}

// TestRenderRejectsEmptyResult verifies render fails when no keypair changed.
func TestRenderRejectsEmptyResult(t *testing.T) {
	t.Parallel()

	err := render(&renderParams{
		Writer: &bytes.Buffer{},
		Result: actionResult{},
	})
	if err == nil {
		t.Fatal("render() succeeded without a rotated keypair")
	}
	if !strings.Contains(err.Error(), "no keypair change") {
		t.Errorf("error = %q, want no-keypair-change guidance", err)
	}
}
