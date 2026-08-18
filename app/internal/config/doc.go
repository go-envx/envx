// Package config is the resolution pipeline: it meshes the user's optional input
// values, ENVX_* environment variables, and the loaded manifest into a single
// resolved *Result, applying the precedence explicit > ENVX_* > project >
// global. It exposes two entry points for the two workflows actions need:
// ResolveProject resolves a project's build-ready configuration, binds a resolver
// factory, and constructs the envmerge Manager the environment-building actions
// operate through; ResolveWorkspace resolves manifest-level data without
// selecting a project, touching the store, or constructing a Manager, for actions
// that locate and edit an overlay. Construction is lazy: New performs no namespace
// or secrets I/O, and each Manager operation opens a fresh, operation-scoped
// resolver from the factory and selects its own environment and reveal policy, so
// no store snapshot or private-key cache survives a call. Each call loads the
// manifest once and returns the aggregate the action reads. It is
// framework-agnostic — no cobra — taking presence as an optional (non-nil) value
// rather than any flag-set handle, so the same pipeline serves a future API. It
// owns the precedence layering and deliberately leaves terminal defaults (such as
// the first-declared environment) unset.
package config
