package file

import (
	"fmt"
	"os"
	"path/filepath"
)

// -------------------------------------------------------------------------------------

// WriteAtomic writes data to path durably: it writes to a temporary file in the
// same directory, fixes the mode to 0600, then renames it over the target. The
// temp+rename dance guarantees a crash never leaves a half-written file in
// place; readers see either the old contents or the complete new contents.
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".envx-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	// Best-effort cleanup: a no-op once the rename below succeeds.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp file %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting permissions on %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", tmpName, path, err)
	}
	return nil
}
