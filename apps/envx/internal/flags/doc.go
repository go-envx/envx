// Package flags is the single source of truth for every envx CLI flag. It is a
// pure leaf package that imports nothing from the rest of the codebase (and no
// CLI framework), so config, engine, cli, and the actions can all read the same
// catalog without pulling in cobra or pflag. Registration happens at the cobra
// edge; this package only *describes* flags.
package flags
