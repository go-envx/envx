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

// TestLoadAndLookup verifies a well-formed store parses and looks up entries,
// and that a missing group or key reports absence.
func TestLoadAndLookup(t *testing.T) {
	t.Parallel()

	path := writeStore(t,
		"secrets:\n  production:\n    postgres_password: prod-pw\n",
	)
	store, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if v, ok := store.Lookup("production", "postgres_password"); !ok || v != "prod-pw" {
		t.Errorf("Lookup = %q, %v; want prod-pw, true", v, ok)
	}
	if _, ok := store.Lookup("production", "missing"); ok {
		t.Error("expected missing key to be absent")
	}
	if _, ok := store.Lookup("ghost", "x"); ok {
		t.Error("expected missing group to be absent")
	}
}

// -------------------------------------------------------------------------------------

// TestLoadMissingFileIsEmpty verifies an absent secrets file yields an empty
// store rather than an error (secrets are optional).
func TestLoadMissingFileIsEmpty(t *testing.T) {
	t.Parallel()

	store, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("Load absent: %v", err)
	}
	if _, ok := store.Lookup("any", "thing"); ok {
		t.Error("empty store should have no entries")
	}
}

// -------------------------------------------------------------------------------------

// TestLoadMalformed verifies a malformed secrets file is an error.
func TestLoadMalformed(t *testing.T) {
	t.Parallel()

	if _, err := Load(writeStore(t, "{")); err == nil {
		t.Error("expected error for malformed secrets file")
	}
}
