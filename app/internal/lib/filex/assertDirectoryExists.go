package filex

import (
	"errors"
	"fmt"
	"os"
)

// ErrNotDirectory indicates a path exists but is not a directory.
var ErrNotDirectory = errors.New("not a directory")

// AssertDirectoryExists returns an error unless path is an existing directory.
func AssertDirectoryExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: %w", path, ErrNotDirectory)
	}
	return nil
}
