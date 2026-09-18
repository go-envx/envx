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
// The bundle is flat: every namespace file is written directly in the output
// directory under its include's final segment (e.g. "env/postgres" becomes
// "postgres.yaml"), and the manifest's includes and secrets path are rewritten to
// match. Two namespaces that share a name are disambiguated with a numeric suffix,
// and names are kept within the filesystem's per-name length limit. Flattening
// removes the source directory structure, so an include that lives outside the
// workspace root is bundled like any other.
package pack
