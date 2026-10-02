// Package core composes the application: the App factory loads a workspace
// manifest and assembles the scoped domain services (env, secrets, pack,
// validate) that CLI commands consume. Construction is lazy and performs no
// namespace or secrets I/O; each env operation opens a fresh, operation-scoped
// resolver, so no store snapshot or private-key cache survives a call. The
// manifest path arrives from the caller (the CLI resolves --config and
// ENVX_CONFIG), and ENVX_* settings reach services through per-operation
// options rather than being read here. It is framework-agnostic — no cobra.
package core
