// Package fixtures resolves absolute paths into the shared test-fixture tree
// (apps/envx/testdata) from a single, fixed location. Test files across packages
// use it instead of computing fragile "../.." offsets that depend on how deep
// the calling package sits in the tree.
package fixtures

import (
	"path/filepath"
	"runtime"
)

// -------------------------------------------------------------------------------------
// root returns the absolute path to apps/envx — the directory above testdata —
// derived from this file's own location (apps/envx/internal/fixtures). Because
// the offset is measured from here, it is the same no matter which package calls
// in.
func root() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("fixtures: cannot determine source location")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// -------------------------------------------------------------------------------------
// Testdata returns the absolute path to the testdata directory, optionally
// joined with a subpath within it. With no argument it points at testdata
// itself: Testdata() -> "<root>/testdata"; Testdata("basic") ->
// "<root>/testdata/basic".
func Testdata(path ...string) string {
	return filepath.Join(append([]string{root(), "testdata"}, path...)...)
}

// -------------------------------------------------------------------------------------
// Manifest returns the absolute path to a fixture project's envx.yaml, e.g.
// Manifest("basic") -> "<root>/testdata/basic/envx.yaml".
func Manifest(name string) string {
	return Testdata(name, "envx.yaml")
}
