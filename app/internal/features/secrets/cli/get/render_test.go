package get

import (
	"bytes"
	"testing"
)

// TestRenderPrintsPlaintext verifies the decrypted value is printed for piping
// and nothing else.
func TestRenderPrintsPlaintext(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := render(&renderParams{
		Writer: &out,
		Result: actionResult{Value: "database-password"},
	})
	if err != nil {
		t.Fatalf("render(): %v", err)
	}
	if out.String() != "database-password\n" {
		t.Errorf("render() = %q, want the plaintext line", out.String())
	}
}
