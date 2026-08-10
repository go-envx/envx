package inspect

import (
	"bytes"
	"testing"

	"github.com/go-envx/envx/app/internal/secrets"
)

// TestRender verifies render writes public metadata and private-key status.
func TestRender(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := render(&renderParams{
		Writer: &output,
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
	if got := output.String(); got != want {
		t.Errorf("render = %q, want %q", got, want)
	}
}
