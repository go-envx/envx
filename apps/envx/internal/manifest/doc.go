// Package manifest discovers, loads, parses, and validates the envx.yaml
// workspace manifest. It owns the on-disk schema and read helpers but knows
// nothing about precedence, CLI flags, or the engine — it is the single,
// frontend-agnostic outlet for reading the manifest file.
package manifest
