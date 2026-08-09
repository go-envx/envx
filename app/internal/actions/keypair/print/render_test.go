package print

import (
	"bytes"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
)

// -------------------------------------------------------------------------------------

// TestRender verifies render writes self-identifying key values with display
// spacing after their type prefixes.
func TestRender(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := render(&renderParams{
		Writer: &output,
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
	if got := output.String(); got != want {
		t.Errorf("render = %q, want %q", got, want)
	}
}
