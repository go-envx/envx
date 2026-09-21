// Package pack selects and copies an environment-scoped subset of a workspace
// into an isolated output directory, ready to be dropped into a container and run
// through the ordinary envx pipeline.
//
// The bundle contains the manifest, each selected project's include files (the
// base namespace file plus only the selected environments' overlays), and a
// filtered secrets store holding only the values the selected environments
// reference. It deliberately excludes the private-key file, which is supplied at
// runtime through ENVX_PRIVATE_KEY, every unselected environment's overlays, and —
// because a bundle only runs, never encrypts — every public key. Nothing is
// decrypted: the encrypted values travel through unchanged, so the copied
// workspace runs through the exact same `envx run --config` that development uses,
// with secrets decrypting at runtime.
//
// The bundle uses a per-project directory layout: each selected project gets its
// own "<project>/" directory holding every namespace file it includes, written
// under the include's final segment (e.g. "env/postgres" becomes "postgres.yaml"
// inside the directory). The manifest's includes are rewritten to
// "<project>/<name>" to match, and its explicit secrets path is dropped. Project
// directory names are sanitized from the arbitrary manifest project names for
// filesystem safety and disambiguated against each other and the reserved root
// filenames; within a single project, two includes that share a final segment are
// disambiguated with a numeric suffix. The project subdirectory is always present,
// including single-project packs, so the layout and consumer command never fork on
// project count.
//
// The bundle follows a two-policy contract:
//
//   - Namespace files are duplicated per project directory. A file two projects
//     both include is copied into each project's directory; there is no
//     cross-project de-duplication. Duplication is intentional and cheap — config
//     files are small and a bundle is a regenerated point-in-time artifact, so
//     there is no edit-drift risk.
//   - The secrets store is a single shared secrets.yaml at the bundle root,
//     filtered to the union of the selected projects' references.
//
// A consequence of that contract is that a multi-project bundle is a
// convenience/organization artifact, not a per-container isolation artifact: the
// per-project directories give organization, collision scoping, and clean
// per-project origins, but the store lives at the root, so a project directory is
// not independently runnable on its own. Per-project secret isolation is achieved
// by re-running pack once per project into a separate output, which yields a store
// filtered to just that project; pack adds no per-project store-partitioning code.
package pack
