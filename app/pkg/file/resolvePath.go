package file

import "path/filepath"

// -------------------------------------------------------------------------------------

// ResolvePath resolves path against base when path is relative. Absolute paths
// are returned cleaned and unchanged; this function does not access the filesystem.
func ResolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}
