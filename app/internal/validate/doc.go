// Package validate implements the workspace-wide enforcement engine behind
// "envx validate".
//
// It aggregates the per-environment dry-run diagnosis across every project and
// environment and adds the store-level findings no single environment can see:
// plaintext values left in the store, orphaned values no environment references,
// and broken or unavailable keypairs. It never materializes plaintext and never
// aborts on a per-value failure; it grades the collected findings into a pass or
// fail verdict a command maps to an exit code. The store-only findings are
// factored apart so the offline audit command can reuse them.
package validate
