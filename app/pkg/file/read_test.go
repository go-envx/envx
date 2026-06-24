package file

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestReadReturnsContents verifies the file contents are returned verbatim.
func TestReadReturnsContents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "in.yaml")
	want := []byte("key: value\n")
	if err := os.WriteFile(target, want, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Read(target)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

// -------------------------------------------------------------------------------------

// TestReadMissing verifies a missing file yields a wrapped os.ErrNotExist.
func TestReadMissing(t *testing.T) {
	t.Parallel()

	_, err := Read(filepath.Join(t.TempDir(), "missing.yaml"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want os.ErrNotExist", err)
	}
}
