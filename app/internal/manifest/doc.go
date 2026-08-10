// Package manifest discovers, loads, parses, and validates the envx.yaml
// workspace manifest. A Manager binds one location — an explicit path or the
// conventional filename to walk up for — and exposes Exists and Load over it,
// but knows nothing about precedence, CLI binding, or envmerge. The declared
// structure it parses (manifest, projects, settings block) lives in the schema
// package; this package adds only the location-aware reads. It is the single,
// frontend-agnostic outlet for reading the manifest file.
package manifest
