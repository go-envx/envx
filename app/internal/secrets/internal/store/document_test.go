package store

import (
	"os"
	"path/filepath"
	"testing"
)

// writeDocument writes a test document and returns its path.
func writeDocument(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestOpenMissingAndMalformed verifies missing documents are empty and malformed
// YAML or document shapes are rejected.
func TestOpenMissingAndMalformed(t *testing.T) {
	t.Parallel()

	document, err := Open(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Open() missing error = %v", err)
	}
	if got := document.Secrets(); len(got) != 0 {
		t.Fatalf("missing document has %d secrets, want 0", len(got))
	}

	for _, body := range []string{"{", "secrets: [not-a-mapping]"} {
		if _, err := Open(writeDocument(t, body)); err == nil {
			t.Errorf("Open(%q) succeeded", body)
		}
	}
}
