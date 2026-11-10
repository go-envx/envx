package cli

import (
	"testing"
)

// TestSecretsGetCommandPrintsPlaintext verifies the decrypted value is printed
// with a trailing newline.
func TestSecretsGetCommandPrintsPlaintext(t *testing.T) {
	t.Parallel()

	stdout, _, err := execute(
		t, seededFactory(t), "", "secrets", "get", "Production", "api_key",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if stdout != "s3cret\n" {
		t.Errorf("stdout = %q, want s3cret", stdout)
	}
}
