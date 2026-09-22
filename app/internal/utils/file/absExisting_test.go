package file

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestAbsExistingResolves verifies an existing relative path resolves to its
// absolute form.
func TestAbsExistingResolves(t *testing.T) {
	// t.Chdir forbids t.Parallel.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "in.yaml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	got, err := AbsExisting("in.yaml")
	if err != nil {
		t.Fatalf("AbsExisting: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

// TestAbsExistingMissing verifies a missing path yields a wrapped os.ErrNotExist.
func TestAbsExistingMissing(t *testing.T) {
	t.Parallel()

	_, err := AbsExisting(filepath.Join(t.TempDir(), "missing.yaml"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want os.ErrNotExist", err)
	}
}
