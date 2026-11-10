package cli

import (
	"strings"
	"testing"
)

// TestKeypairGenerateCommand verifies the new keypair is retained and reported
// without its private key.
func TestKeypairGenerateCommand(t *testing.T) {
	t.Parallel()

	factory := newFakeFactory()
	stdout, _, err := execute(t, factory, "", "keypair", "generate", "staging")
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	retained := factory.keys.byPublic["staging"]
	if len(retained) != 1 {
		t.Fatalf("retained keys = %d, want 1", len(retained))
	}
	for public, private := range retained {
		for _, want := range []string{
			`Generated keypair for group "staging":`,
			"public key: " + public,
			"secrets store: mem://secrets",
			"private key file: mem://keys",
		} {
			if !strings.Contains(stdout, want) {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}
		}
		if strings.Contains(stdout, private) {
			t.Error("stdout leaked the private key")
		}
	}
}
