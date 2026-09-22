package file

import (
	"os"
	"path/filepath"
)

// AbsExisting returns the cleaned, absolute form of path. It returns an error if
// the path does not exist or its absolute form cannot be resolved.
func AbsExisting(path string) (string, error) {
	clean := filepath.Clean(path)
	if _, err := os.Stat(clean); err != nil {
		return "", err
	}
	return filepath.Abs(clean)
}
