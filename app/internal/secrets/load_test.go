package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------

// writeStore writes body to a secrets.yaml in a fresh temp dir and returns its path.
func writeStore(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// -------------------------------------------------------------------------------------

// TestLoadAndLookup verifies a well-formed store parses and flattens into
// reference-keyed entries, and that a missing group or key reports absence.
func TestLoadAndLookup(t *testing.T) {
	t.Parallel()

	path := writeStore(t,
		"secrets:\n  production:\n    postgres_password: prod-pw\n",
	)
	s, err := loadStore(path)
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}

	ref := reference{group: "production", key: "postgres_password"}
	if v, ok := s.lookup(ref); !ok || v != "prod-pw" {
		t.Errorf("lookup = %q, %v; want prod-pw, true", v, ok)
	}
	if _, ok := s.lookup(reference{group: "production", key: "missing"}); ok {
		t.Error("expected missing key to be absent")
	}
	if _, ok := s.lookup(reference{group: "ghost", key: "x"}); ok {
		t.Error("expected missing group to be absent")
	}
}

// -------------------------------------------------------------------------------------

// TestLoadMissingFileIsEmpty verifies an absent secrets file yields an empty
// store rather than an error (secrets are optional).
func TestLoadMissingFileIsEmpty(t *testing.T) {
	t.Parallel()

	s, err := loadStore(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("loadStore absent: %v", err)
	}
	if _, ok := s.lookup(reference{group: "any", key: "thing"}); ok {
		t.Error("empty store should have no entries")
	}
}

// -------------------------------------------------------------------------------------

// TestLoadMalformed verifies a malformed secrets file is an error.
func TestLoadMalformed(t *testing.T) {
	t.Parallel()

	if _, err := loadStore(writeStore(t, "{")); err == nil {
		t.Error("expected error for malformed secrets file")
	}
}
