package filex

import (
	"path/filepath"
	"strings"
)

// RelativeTo returns path relative to base when path lies inside base. Paths
// outside base are returned unchanged; this function does not access the filesystem.
func RelativeTo(base, path string) string {
	relativePath, err := filepath.Rel(base, path)
	filePathSeparator := string(filepath.Separator)
	prefix := ".." + filePathSeparator
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, prefix) {
		return path
	}
	return relativePath
}
