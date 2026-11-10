package testutil

import (
	"io/fs"
	"os"
	"runtime"
	"testing"
)

// AssertPerm fails tb unless the file at path has permission bits want. Windows
// has no POSIX permission bits, so the check is skipped there.
func AssertPerm(tb testing.TB, path string, want fs.FileMode) {
	tb.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		tb.Fatalf("testutil: stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		tb.Errorf("%s mode = %o, want %o", path, got, want)
	}
}
