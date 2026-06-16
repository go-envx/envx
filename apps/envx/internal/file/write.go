package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/str"
)

// -------------------------------------------------------------------------------------
// Write creates a file at dir/name with the given content. The content is
// automatically dedented so callers can use indented Go backtick strings
// that align with surrounding code.
func Write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(
		filepath.Join(dir, name),
		[]byte(str.Dedent(content)),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
}
