package file

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotFound reports that filename was not located in the start directory or
// any of its ancestors within the search boundary. Callers can branch on it
// with errors.Is to supply their own context-specific message.
var ErrNotFound = errors.New("file not found in directory tree")

// FindUp walks up from startDir looking for filename in each directory,
// returning the absolute path to the first match. The search begins at startDir
// and ascends through its parents. It stops at the filesystem root, or — when a
// directory contains any of the boundary markers — at that directory. The
// boundary is inclusive: the directory holding a marker is still searched for
// filename before the walk halts. It returns ErrNotFound if no match exists
// within the boundary.
func FindUp(startDir, filename string, boundaries ...string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving start directory %q: %w", startDir, err)
	}

	for {
		candidate := filepath.Join(dir, filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		if hasBoundary(dir, boundaries) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ErrNotFound
}

// hasBoundary reports whether dir contains any of the named boundary markers,
// each of which halts the upward search at the current directory.
func hasBoundary(dir string, boundaries []string) bool {
	for _, marker := range boundaries {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}
