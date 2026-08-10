package fixtures

import (
	"path/filepath"
	"runtime"
)

// root returns the absolute path to app
func root() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("fixtures: cannot determine source location")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// Testdata returns the absolute path to the testdata directory, optionally
// joined with a subpath within it.
//   - Testdata() -> "<root>/testdata"
//   - Testdata("basic") -> "<root>/testdata/basic"
//   - Testdata("basic", "envx.yaml") -> "<root>/testdata/basic/envx.yaml"
func Testdata(path ...string) string {
	return filepath.Join(append([]string{root(), "testdata"}, path...)...)
}

// Manifest returns the absolute path to a fixture project's envx.yaml, e.g.
//   - Manifest("basic") -> "<root>/testdata/basic/envx.yaml"
//   - Manifest("other") -> "<root>/testdata/other/envx.yaml"
func Manifest(name string) string {
	return Testdata(name, "envx.yaml")
}
