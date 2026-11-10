package cli

import (
	"strings"
	"testing"
)

// plaintextFactory returns seededFactory with two plaintext values, a and b,
// added to the "production" group.
func plaintextFactory(t *testing.T) *fakeFactory {
	t.Helper()
	f := seededFactory(t)
	values := f.store.groups["production"].Values
	values["a"] = "plain-a"
	values["b"] = "plain-b"
	return f
}

// TestSecretsEncryptCommand verifies --group and --key narrow the operation and
// --verbose lists each encrypted secret.
func TestSecretsEncryptCommand(t *testing.T) {
	t.Parallel()

	factory := plaintextFactory(t)
	stdout, _, err := execute(
		t, factory, "",
		"secrets", "encrypt", "--group", "production", "--key", "a", "--verbose",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	got := factory.stored("production", "a")
	if !strings.HasPrefix(got, "encrypted-age:") {
		t.Errorf("a = %q, want an age envelope", got)
	}
	if got := factory.stored("production", "b"); got != "plain-b" {
		t.Errorf("b = %q, want it left as plaintext", got)
	}
	for _, want := range []string{"Encrypted 1 secret in:", "  production/a"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
	}
}

// TestSecretsEncryptCommandSummary verifies the summary omits identities
// without --verbose and reports when nothing needed encrypting.
func TestSecretsEncryptCommandSummary(t *testing.T) {
	t.Parallel()

	stdout, _, err := execute(t, plaintextFactory(t), "", "secrets", "encrypt")
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if !strings.Contains(stdout, "Encrypted 2 secrets in:") ||
		strings.Contains(stdout, "production/a") {
		t.Errorf("stdout = %q, want a count without identities", stdout)
	}

	stdout, _, err = execute(t, seededFactory(t), "", "secrets", "encrypt")
	if err != nil {
		t.Fatalf("Execute() with nothing to encrypt: %v", err)
	}
	if !strings.Contains(stdout, "No plaintext values to encrypt.") {
		t.Errorf("stdout = %q, want the nothing-to-encrypt message", stdout)
	}
}
