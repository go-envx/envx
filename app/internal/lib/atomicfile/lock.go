package atomicfile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// ErrLockTimeout indicates that acquiring the file lock timed out or could not
// be completed.
var ErrLockTimeout = errors.New("lock acquisition failed")

// FileLock represents an acquired advisory lock on a file path.
type FileLock struct {
	path string
	file *os.File
}

// lockMu serializes in-process lock acquisitions per canonical path to ensure
// goroutine safety within the same process.
var (
	processLocks   = make(map[string]*sync.Mutex)
	processLocksMu sync.Mutex
)

func getPathMutex(canonicalPath string) *sync.Mutex {
	processLocksMu.Lock()
	defer processLocksMu.Unlock()
	mu, ok := processLocks[canonicalPath]
	if !ok {
		mu = &sync.Mutex{}
		processLocks[canonicalPath] = mu
	}
	return mu
}

// Lock acquires an exclusive lock on path (using an adjacent lockfile at
// path + ".lock"). It blocks until the lock is acquired or an error occurs.
func Lock(path string) (*FileLock, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving absolute path %s: %w", path, err)
	}

	mu := getPathMutex(absPath)
	mu.Lock()

	lockFilePath := filepath.Clean(absPath + ".lock")
	//nolint:gosec // G304: lock file path is constructed from validated target path
	f, err := os.OpenFile(lockFilePath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		mu.Unlock()
		return nil, fmt.Errorf("opening lock file %s: %w", lockFilePath, err)
	}

	if err := lockFile(f); err != nil {
		_ = f.Close()
		mu.Unlock()
		return nil, fmt.Errorf("locking %s: %w", lockFilePath, err)
	}

	return &FileLock{
		path: absPath,
		file: f,
	}, nil
}

// Unlock releases the file lock and closes the lockfile descriptor.
func (fl *FileLock) Unlock() error {
	if fl == nil || fl.file == nil {
		return nil
	}

	var unlockErr error
	if err := unlockFile(fl.file); err != nil {
		unlockErr = fmt.Errorf("unlocking file %s: %w", fl.file.Name(), err)
	}

	if closeErr := fl.file.Close(); closeErr != nil && unlockErr == nil {
		unlockErr = fmt.Errorf("closing lock file %s: %w", fl.file.Name(), closeErr)
	}
	fl.file = nil

	mu := getPathMutex(fl.path)
	mu.Unlock()

	return unlockErr
}

// ReadModifyWrite executes fn under an exclusive lock on path.
// It reads the current contents of path (or an empty byte slice if not found),
// executes fn with the read data, and if fn returns without error, atomically
// writes the updated data back to path.
func ReadModifyWrite(path string, fn func(data []byte) ([]byte, error)) error {
	fl, err := Lock(path)
	if err != nil {
		return fmt.Errorf("locking for read-modify-write on %s: %w", path, err)
	}
	defer func() { _ = fl.Unlock() }()

	var current []byte
	cleanPath := filepath.Clean(path)
	//nolint:gosec // G304: callers provide validated config-derived paths
	f, err := os.Open(cleanPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reading current file %s: %w", path, err)
		}
		current = nil
	} else {
		defer func() { _ = f.Close() }()
		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("reading file contents %s: %w", path, err)
		}
		current = data
	}

	updated, err := fn(current)
	if err != nil {
		return err
	}

	if err := Write(path, updated); err != nil {
		return fmt.Errorf("writing updated contents to %s: %w", path, err)
	}

	return nil
}
