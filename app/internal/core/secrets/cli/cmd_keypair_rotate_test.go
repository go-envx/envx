package cli

import (
	"strings"
	"testing"
)

// TestKeypairRotateCommand verifies the group is re-keyed and the summary
// reports the re-encrypted count and new public key.
func TestKeypairRotateCommand(t *testing.T) {
	t.Parallel()

	factory := seededFactory(t)
	before := factory.store.groups["production"].PublicKey
	stdout, _, err := execute(t, factory, "", "keypair", "rotate", "production")
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	after := factory.store.groups["production"].PublicKey
	if after == before {
		t.Error("public key was not rotated")
	}
	for _, want := range []string{
		`Rotated keypair for group "production" and re-encrypted 1 secret:`,
		"public key: " + after,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
	}
}
