package envmerge

import "slices"

// defaultDelimiter joins list-valued leaves when no delimiter is configured.
const defaultDelimiter = ","

// Params is envmerge's input contract. The caller supplies namespace paths, the
// precedence-resolved default environment, settings, and optional value-resolution
// behavior without exposing where they came from. envmerge fills in terminal
// defaults itself.
type Params struct {
	// Includes is an ordered chain of namespaces to merge, given as absolute paths
	// the caller has already resolved.
	Includes []string
	// Environments lists the declared environments, used to validate the target.
	Environments []string
	// DefaultEnvironment is the precedence-resolved default an operation uses when
	// it is given no explicit environment. It is not validated at construction
	// because an explicit operation environment supersedes it.
	DefaultEnvironment string
	// Settings holds the fully-resolved env-resolution knobs the merge reads.
	Settings Settings
	// ResolverFactory opens a fresh, operation-scoped value resolver on demand. A
	// nil factory is identity behavior for callers with no reference syntax.
	ResolverFactory ValueResolverFactory
}

// ValueResolver dereferences one winning scalar value and returns unrecognized
// values unchanged. env is the active environment, used by implementations that
// support environment-implicit references.
type ValueResolver interface {
	// Resolve returns value with any recognized reference dereferenced, or value
	// unchanged when it is not a reference.
	Resolve(value, env string) (string, error)
}

// ValueResolverFactory opens a fresh, operation-scoped value resolver under the
// requested reveal policy. Each resolving operation asks for a new resolver after
// namespace winner selection, so no store snapshot or private-key cache survives
// the operation.
type ValueResolverFactory interface {
	// Resolver returns a fresh resolver materializing references under the reveal
	// policy: a revealing resolver decrypts, a masking resolver returns canonical
	// references.
	Resolver(reveal bool) (ValueResolver, error)
}

// Settings holds the fully-resolved env-resolution knobs envmerge consumes — a
// plain value struct with no knowledge of how its values were sourced, and no
// environment or reveal policy, which are per-operation concerns. Zero values are
// valid: the bool/string knobs default to off and an empty delimiter falls back
// to the default (",").
type Settings struct {
	// RequireOverlays requires each namespace's environment overlay file to exist.
	RequireOverlays bool
	// Prefix is prepended to every resolved key.
	Prefix string
	// Suffix is appended to every resolved key.
	Suffix string
	// Delimiter joins a list-valued leaf into one string; an empty value means
	// the default (",") that normalizeParams applies.
	Delimiter string
	// NamespacePrefix prefixes each resolved key with its namespace name.
	NamespacePrefix bool
}

// normalizeParams applies envmerge's structural terminal defaults, copies the
// caller-owned slices so later mutation cannot change manager behavior, and
// returns the normalized params. It does not validate the environment: each
// operation validates the environment it actually uses, so an irrelevant default
// cannot block an operation that overrides it.
func normalizeParams(params Params) (Params, error) {
	// Apply the default list delimiter when none was configured.
	if params.Settings.Delimiter == "" {
		params.Settings.Delimiter = defaultDelimiter
	}

	// Copy caller-owned slices so caller mutation cannot change manager behavior.
	params.Includes = slices.Clone(params.Includes)
	params.Environments = slices.Clone(params.Environments)

	// No structural defaults fail today; the error result keeps the contract
	// stable for future validation.
	return params, nil
}
