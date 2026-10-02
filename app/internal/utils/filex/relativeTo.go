package filex

import (
	"path/filepath"
	"strings"
)

// RelativeTo returns path relative to base when path lies inside base. Paths
// outside base are returned unchanged; this function does not access the filesystem.
func RelativeTo(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}
