package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteFilteredStore verifies the bundle store keeps only referenced values,
// matches groups case-insensitively, and drops every public key.
func TestWriteFilteredStore(t *testing.T) {
	t.Parallel()

	src := filepath.Join(t.TempDir(), "secrets.yaml")
	body := "public_keys:\n  Shared: pub\n" +
		"secrets:\n" +
		"  Shared:\n    token: t0k\n    unused: nope\n" +
		"  other:\n    x: y\n"
	if err := os.WriteFile(src, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "out.yaml")

	// The reference lowercases the group ("shared"), while the store spells it
	// "Shared"; the filter must still match it.
	err := WriteFilteredStore(src, dst, []SecretReference{{Group: "shared", Key: "token"}})
	if err != nil {
		t.Fatalf("WriteFilteredStore: %v", err)
	}

	got, err := os.ReadFile(dst) //nolint:gosec // path is test-local.
	if err != nil {
		t.Fatal(err)
	}
	out := string(got)
	if !strings.Contains(out, "token: t0k") {
		t.Errorf("referenced secret missing:\n%s", out)
	}
	gone := []string{"unused", "nope", "other", "x: y", "public_keys", "pub"}
	for _, dropped := range gone {
		if strings.Contains(out, dropped) {
			t.Errorf("filtered store still contains %q:\n%s", dropped, out)
		}
	}
}
