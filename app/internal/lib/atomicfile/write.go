package atomicfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Write writes data to path durably, preserving an existing file's
// permission bits or using 0644 for a new file. It writes to a temporary file in
// the same directory, flushes data to disk with Sync, and atomically renames it
// over the target.
func Write(path string, data []byte) error {
	perm, err := fileMode(path)
	if err != nil {
		return err
	}
	return write(path, data, perm)
}

// WritePrivate writes data to path durably with 0600 permission bits. It always
// applies the private mode, including when replacing an existing file.
func WritePrivate(path string, data []byte) error {
	return write(path, data, 0o600)
}

// WriteAtomic is an alias for Write to ease transition from utils/filex.
func WriteAtomic(path string, data []byte) error {
	return Write(path, data)
}

// WriteAtomicPrivate is an alias for WritePrivate to ease transition from utils/filex.
func WriteAtomicPrivate(path string, data []byte) error {
	return WritePrivate(path, data)
}

// fileMode returns the existing permission bits for path, or the deterministic
// default for a path that does not exist yet.
func fileMode(path string) (os.FileMode, error) {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		return info.Mode().Perm(), nil
	case errors.Is(err, fs.ErrNotExist):
		return 0o644, nil
	default:
		return 0, fmt.Errorf("checking permissions on %s: %w", path, err)
	}
}

// write writes data to a temporary file with perm, fsyncs it, and renames it over
// path after the write is complete.
func write(path string, data []byte, perm os.FileMode) error {
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
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting permissions on %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("syncing temp file %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", tmpName, path, err)
	}
	return nil
}
