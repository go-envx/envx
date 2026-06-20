// Package fixtures is a small collection of helpers for tapping into well-known
// local directories from any package, regardless of how deep the caller sits in
// the tree. It hands back stable absolute paths (e.g. the testdata fixture tree)
// so tests never rely on fragile "../.." offsets.
package fixtures
