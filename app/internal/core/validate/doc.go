// Package validate grades a whole workspace into a pass or fail report.
//
// It diagnoses every project against every declared environment through a
// masked explanation and adds the store-level findings no single environment
// can see: plaintext or foreign-algorithm values, values no environment
// references, groups missing a public key, and broken or unavailable private
// keys. It never materializes plaintext and never aborts on a per-value
// problem; operational failures are errors, diagnosed problems are findings.
//
// Selected checks bound the cost of a run: the environment explainer is opened
// only when a selected check reads the merge, and the secrets inspector only
// when a selected check reads the store, auditing just the sections those
// checks need.
//
// # Ownership
//
// The env explanation and the store audit both observe store-centric problems:
// a reference to a plaintext value, for example, carries SECRET_IS_NOT_ENCRYPTED
// from the resolver while the audit flags the stored value itself. Each code is
// owned by its catalog group, and only the owner reports it, so a problem is
// reported once per stored value or group rather than once per reference:
//
//   - store: SECRET_IS_NOT_ENCRYPTED, SECRET_ALGORITHM_MISMATCH,
//     SECRET_IS_NOT_REFERENCED, PUBLIC_KEY_IS_MISSING, PRIVATE_KEY_IS_INVALID,
//     and PRIVATE_KEY_IS_UNAVAILABLE come only from the audit.
//   - provider: EXTERNAL_PROVIDER_UNAVAILABLE comes only from the offline
//     provider check, once per declared provider some winner references.
//   - resolution: reference and substitution codes, plus
//     PROPERTY_NOT_DECLARED_IN_BASE, come only from the explanation.
//
// Findings are further deduplicated by code and subject, then sorted errors
// first, by group, code, project, environment, and key.
package validate
