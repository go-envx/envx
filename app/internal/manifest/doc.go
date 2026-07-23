// Package manifest discovers, loads, parses, and validates the envx.yaml
// workspace manifest. It owns the on-disk concerns — locating the file and
// pairing the parsed schema.Manifest with its directory as a *Loaded — but knows
// nothing about precedence, CLI binding, or envmerge. The declared structure it
// parses (manifest, projects, settings block) lives in the schema package; this
// package adds only the location-aware reads. It is the single, frontend-agnostic
// outlet for reading the manifest file.
package manifest
