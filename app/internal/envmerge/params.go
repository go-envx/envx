package envmerge

import (
	"fmt"
	"slices"
)

// -------------------------------------------------------------------------------------

// defaultDelimiter joins list-valued leaves when no delimiter is configured.
const defaultDelimiter = ","

// -------------------------------------------------------------------------------------

// Params is envmerge's input contract: a plain data bag the caller fully
// populates. envmerge knows nothing about where its inputs came from or how they
// were produced. Every field is optional and envmerge fills in terminal defaults
// itself. The resolved knobs live in Settings, the value form envmerge merges.
type Params struct {
	// Includes is an ordered chain of namespaces to merge, given as absolute paths
	// the caller has already resolved.
	Includes []string
	// Environments lists the declared environments, used to validate the target.
	Environments []string
	// Settings holds the fully-resolved env-resolution knobs the merge reads.
	Settings Settings
	// Resolver dereferences reference-valued leaves (e.g. secrets) after the merge.
	// A nil Resolver leaves every value untouched, so callers with no references
	// need not supply one.
	Resolver Resolver
}

// -------------------------------------------------------------------------------------

// Resolver dereferences a single merged value, substituting any reference it
// recognizes and returning plain values unchanged. env is the active
// environment, used to resolve environment-implicit references. envmerge defines
// this interface (the consumer); an implementation lives in the secrets package.
type Resolver interface {
	// Resolve returns value with any recognized reference dereferenced, or value
	// unchanged when it is not a reference.
	Resolve(value, env string) (string, error)
}

// -------------------------------------------------------------------------------------

// Settings holds the fully-resolved env-resolution knobs envmerge consumes — a
// plain value struct with no knowledge of how its values were sourced. Zero values
// are valid: an empty Env falls back to the first declared environment and the
// bool/string knobs default to off.
type Settings struct {
	// Env is the target environment to resolve; an empty value falls back to the
	// first declared environment.
	Env string
	// Strict requires each namespace's environment overlay file to exist.
	Strict bool
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

// -------------------------------------------------------------------------------------

// normalizeParams applies envmerge's terminal defaults to Params and validates the
// result. It mutates Params in place so Build can read the effective settings directly.
func normalizeParams(p *Params) error {
	// Apply the first declared environment as the default if the target is empty.
	if p.Settings.Env == "" && len(p.Environments) > 0 {
		p.Settings.Env = p.Environments[0]
	}

	// Apply the default list delimiter when none was configured.
	if p.Settings.Delimiter == "" {
		p.Settings.Delimiter = defaultDelimiter
	}

	// Validate the resolved environment against the declared set.
	if !slices.Contains(p.Environments, p.Settings.Env) {
		return fmt.Errorf(
			"environment %q is not declared (available: %v)",
			p.Settings.Env, p.Environments,
		)
	}

	// No error means Params are normalized and valid.
	return nil
}
