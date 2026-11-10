package cli

import (
	"strings"
	"testing"
)

// TestKeypairInspectCommand verifies the public key and private-key status are
// reported.
func TestKeypairInspectCommand(t *testing.T) {
	t.Parallel()

	factory := seededFactory(t)
	stdout, _, err := execute(t, factory, "", "keypair", "inspect", "production")
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	public := factory.store.groups["production"].PublicKey
	for _, want := range []string{
		`Keypair for group "production":`,
		"public key: " + public,
		"private key: valid",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
	}
}
