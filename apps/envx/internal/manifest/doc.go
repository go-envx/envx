// Package manifest discovers, loads, parses, and validates the envx.yaml
// workspace manifest. It owns the on-disk structure and read helpers but knows
// nothing about precedence, CLI binding, or the engine; the settings blocks it
// parses use the shared settings.File schema. It is the single, frontend-agnostic
// outlet for reading the manifest file.
package manifest
