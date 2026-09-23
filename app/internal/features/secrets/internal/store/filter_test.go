package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// keepOnly returns a keep predicate that retains exactly the given "group/key"
// pairs, matching the group case-insensitively as resolution does.
func keepOnly(pairs ...string) func(group, key string) bool {
	set := make(map[string]bool, len(pairs))
	for _, p := range pairs {
		set[strings.ToLower(p)] = true
	}
	return func(group, key string) bool {
		return set[strings.ToLower(group)+"/"+key]
	}
}

// TestRetainSecretsKeepsSubset verifies retention keeps only the named values,
// drops an emptied group, and leaves other groups intact.
func TestRetainSecretsKeepsSubset(t *testing.T) {
	t.Parallel()
	path := writeDocument(t,
		"secrets:\n"+
			"  shared:\n    token: t0k\n    unused: nope\n"+
			"  other:\n    x: y\n")
	document, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := document.RetainSecrets(keepOnly("shared/token")); err != nil {
		t.Fatalf("RetainSecrets: %v", err)
	}

	if _, ok := document.Secret("shared", "token"); !ok {
		t.Error("retained secret shared/token was dropped")
	}
	if _, ok := document.Secret("shared", "unused"); ok {
		t.Error("unreferenced secret shared/unused was kept")
	}
	if _, ok := document.Secret("other", "x"); ok {
		t.Error("unreferenced group other was kept")
	}
}

// TestRetainSecretsDropsEmptyBlock verifies retaining nothing removes the whole
// secrets block rather than leaving an empty mapping.
func TestRetainSecretsDropsEmptyBlock(t *testing.T) {
	t.Parallel()
	path := writeDocument(t, "secrets:\n  shared:\n    token: t0k\n")
	document, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := document.RetainSecrets(keepOnly()); err != nil {
		t.Fatalf("RetainSecrets: %v", err)
	}
	if got := document.Secrets(); len(got) != 0 {
		t.Errorf("Secrets() = %v, want none", got)
	}
}

// TestRemovePublicKeys verifies the public_keys block is removed and is a no-op
// when absent.
func TestRemovePublicKeys(t *testing.T) {
	t.Parallel()
	path := writeDocument(t,
		"public_keys:\n  shared: pub\nsecrets:\n  shared:\n    token: t0k\n")
	document, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := document.RemovePublicKeys(); err != nil {
		t.Fatalf("RemovePublicKeys: %v", err)
	}
	if got := document.PublicKeyGroups(); len(got) != 0 {
		t.Errorf("PublicKeyGroups() = %v, want none", got)
	}
	// A second call, with the block already gone, is a no-op.
	if err := document.RemovePublicKeys(); err != nil {
		t.Fatalf("RemovePublicKeys second call: %v", err)
	}
}

// TestSaveToWritesFilteredCopy verifies SaveTo writes to a different path with
// owner-only permissions, leaving the source untouched.
func TestSaveToWritesFilteredCopy(t *testing.T) {
	t.Parallel()
	src := writeDocument(t,
		"public_keys:\n  shared: pub\n"+
			"secrets:\n  shared:\n    token: t0k\n    unused: nope\n")
	document, err := Open(src)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := document.RetainSecrets(keepOnly("shared/token")); err != nil {
		t.Fatalf("RetainSecrets: %v", err)
	}
	if err := document.RemovePublicKeys(); err != nil {
		t.Fatalf("RemovePublicKeys: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out.yaml")
	if err := document.SaveTo(dst, 2); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want 600", perm)
	}
	written, err := Open(dst)
	if err != nil {
		t.Fatalf("Open written: %v", err)
	}
	if _, ok := written.Secret("shared", "token"); !ok {
		t.Error("written store missing shared/token")
	}
	if _, ok := written.Secret("shared", "unused"); ok {
		t.Error("written store kept shared/unused")
	}
}
