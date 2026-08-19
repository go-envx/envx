package str

import (
	"strconv"
	"strings"
)

// QuotePath quotes a path only when it contains a space, so its boundary stays
// unambiguous in terminal output while unspaced paths remain copy-paste clean.
func QuotePath(path string) string {
	if strings.Contains(path, " ") {
		return strconv.Quote(path)
	}
	return path
}
