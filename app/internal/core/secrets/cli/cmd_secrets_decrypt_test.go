package cli

import (
	"strings"
	"testing"
)

// TestSecretsDecryptCommand verifies --group and --key select the secret,
// which is rewritten as plaintext and listed under --verbose.
func TestSecretsDecryptCommand(t *testing.T) {
	t.Parallel()

	factory := seededFactory(t)
	stdout, _, err := execute(
		t, factory, "",
		"secrets", "decrypt", "--group", "production", "--key", "api_key", "-v",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if got := factory.stored("production", "api_key"); got != "s3cret" {
		t.Errorf("api_key = %q, want the plaintext", got)
	}
	for _, want := range []string{"Decrypted 1 secret in:", "  production/api_key"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
	}
}

// TestSecretsDecryptCommandUnavailableGroup verifies a group without a private
// key is skipped with a warning on stderr.
func TestSecretsDecryptCommandUnavailableGroup(t *testing.T) {
	t.Parallel()

	factory := seededFactory(t)
	clear(factory.keys.byPublic)
	stdout, stderr, err := execute(t, factory, "", "secrets", "decrypt")
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if !strings.Contains(stderr, `no private key available for group "production"`) {
		t.Errorf("stderr = %q, want the unavailable-group warning", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing when no secret was decrypted", stdout)
	}
	if got := factory.stored("production", "api_key"); got == "s3cret" {
		t.Error("api_key was decrypted without a private key")
	}
}
