package file

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------
// TestWriteAtomicCreates verifies a new file is created with 0600 permissions
// and the exact contents.
func TestWriteAtomicCreates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "out.yaml")
	want := []byte("key: value\n")

	if err := WriteAtomic(target, want); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	got, err := os.ReadFile(target) //nolint:gosec // path is a test-owned temp file
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("contents = %q, want %q", got, want)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want 600", perm)
	}
}

// -------------------------------------------------------------------------------------
// TestWriteAtomicOverwrites verifies an existing file is fully replaced and no
// temp artifacts are left behind in the directory.
func TestWriteAtomicOverwrites(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "out.yaml")

	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(target, []byte("new")); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	got, err := os.ReadFile(target) //nolint:gosec // path is a test-owned temp file
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("contents = %q, want %q", got, "new")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, found %d (temp leak?)", len(entries))
	}
}
