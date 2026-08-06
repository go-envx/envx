package privatekey

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestFileDestinationUpdatesExistingEntry verifies an update preserves the rest
// of the private-key file.
func TestFileDestinationUpdatesExistingEntry(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "envx.keys")
	content := "# header\nPRODUCTION=old-value\nSHARED=shared-value\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := NewFileDestination(path).Write("production", "new-value"); err != nil {
		t.Fatalf("Write(): %v", err)
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is test-local.
	if err != nil {
		t.Fatal(err)
	}
	want := "# header\nPRODUCTION=new-value\nSHARED=shared-value\n"
	if string(data) != want {
		t.Errorf("file contents = %q, want %q", data, want)
	}
}

// -------------------------------------------------------------------------------------

// TestFileDestinationRejectsEmptyPath verifies a missing destination path fails
// before any file operation is attempted.
func TestFileDestinationRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	err := NewFileDestination("").Write("production", "private-value")
	if err == nil {
		t.Fatal("Write() accepted an empty file path")
	}
	if got := err.Error(); got != "private-key file path is empty" {
		t.Errorf("Write() error = %q, want empty-path error", got)
	}
}

// -------------------------------------------------------------------------------------

// TestFileDestinationRejectsMalformedExistingFile verifies existing malformed
// content is not replaced by a new private-key entry.
func TestFileDestinationRejectsMalformedExistingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "envx.keys")
	content := []byte("not-an-entry\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := NewFileDestination(path).Write("production", "private-value"); err == nil {
		t.Fatal("Write() accepted malformed existing content")
	} else {
		want := "malformed private-key entry in " + path + " at line 1"
		if err.Error() != want {
			t.Errorf("Write() error = %q, want %q", err, want)
		}
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is test-local.
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("file contents = %q, want unchanged content %q", data, content)
	}
}
