// Package pack copies an environment-scoped subset of a workspace into an
// isolated bundle that runs through the ordinary envx pipeline.
//
// A bundle holds the manifest, each selected project's namespace files (the
// base file plus only the selected environments' overlays), and a secrets store
// filtered to the values those files reference. It excludes private keys,
// public keys, and unselected overlays, and decrypts nothing: ciphertext travels
// unchanged and decrypts at runtime with a key supplied through the environment.
//
// Each selected project gets its own "<project>/" directory holding every
// namespace file it includes, named after the include's final segment ("env/pg"
// becomes "pg.yaml"). Manifest includes are rewritten to "<project>/<name>", and
// an explicit secrets path is dropped so the store resolves to the default
// secrets.yaml at the bundle root. Directory names are sanitized from project
// names and disambiguated against each other and the root bundle files; two
// includes in one project that share a final segment get a numeric suffix. A
// namespace two projects include is copied into each project's directory, while
// the store is shared at the root, so a project directory is not independently
// runnable. Packing one project per output yields per-project secret isolation.
//
// # Pipeline
//
// Pack runs the pipeline_* stages in order and writes nothing until the last:
//
//  1. select: requested environments and projects are resolved against the
//     declared sets, preserving declared order.
//  2. plan: each project's base and overlay files are discovered, bundle
//     directory and file names are assigned, and every source is read once.
//  3. manifest: the manifest is rewritten in place on its YAML node tree.
//  4. secrets: referenced values are read raw from the store and exported.
//  5. publish: the bundle is staged in a fixed scratch directory, then its
//     entries replace an existing directory's entries, or it replaces any
//     other destination path. Every failure restores the previous output, or
//     leaves it in the scratch directory, which blocks later packs until
//     recovered.
//
// Before reading sources, Pack rejects a destination that is, or is an
// ancestor of, the manifest, the secrets store, the key file, or any
// project's include directory, comparing file identity so symlinks,
// case-insensitive names, and bind mounts cannot alias a source.
package pack
