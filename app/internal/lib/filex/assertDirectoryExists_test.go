package filex

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestAssertDirectoryExists verifies only an existing directory passes.
func TestAssertDirectoryExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := AssertDirectoryExists(dir); err != nil {
		t.Errorf("AssertDirectoryExists(dir) = %v, want nil", err)
	}
	if err := AssertDirectoryExists(file); !errors.Is(err, ErrNotDirectory) {
		t.Errorf("AssertDirectoryExists(file) = %v, want ErrNotDirectory", err)
	}
	missing := filepath.Join(dir, "missing")
	if err := AssertDirectoryExists(missing); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("AssertDirectoryExists(missing) = %v, want os.ErrNotExist", err)
	}
}
