// Package env owns environment workflows: get, set, explain, diff,
// materialization, run, and emit. Namespace files, value resolution, process
// execution, and output formats are supplied by adapters.
//
// # Value pipeline
//
// The pipeline_* files hold the value pipeline every read operation shares.
// Building the merged environment is eager:
//
//  1. flatten: each namespace base and overlay file becomes upper-snake keys
//     with unresolved scalar or list leaves and their dotted source paths.
//  2. merge: overlays layer over bases, namespaces layer in declaration order
//     (last wins), and prefix/suffix affixes apply. Each key records its winning
//     source and the sources it shadowed.
//  3. host: host variables then override namespace values, or are shadowed by
//     them under overload. Host values are opaque and never resolved or
//     substituted.
//
// Reading values from the merged environment is lazy and per key, so an
// unreferenced failure never blocks another key:
//
//  4. evaluate: diagnoses one key without aborting, masks secrets unless
//     revealed, and tracks whether a value derives from a secret.
//  5. substitute: composes {{ }} references, host first unless overload,
//     memoizing each key and reporting cycles and missing references.
//  6. resolve: dereferences a key's leaf items through the ValueResolver and
//     renders lists with the delimiter; substitute reaches it for each
//     referenced key.
//
// Each lazy stage pulls from the next. Get and Materialize (and so Run) call
// substitute directly; Explain, Diff, and Emit go through evaluate.
package env
