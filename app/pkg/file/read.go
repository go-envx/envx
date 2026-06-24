package file

import (
	"os"
	"path/filepath"
)

// -------------------------------------------------------------------------------------

// Read returns the contents of the file at path. The path is cleaned before
// access; callers must supply paths derived from validated configuration rather
// than untrusted input.
func Read(path string) ([]byte, error) {
	clean := filepath.Clean(path)
	//nolint:gosec // G304: callers pass config-derived paths, not raw user input
	return os.ReadFile(clean)
}
