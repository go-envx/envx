package envmerge

import (
	"fmt"
	"slices"
)

// -------------------------------------------------------------------------------------

// Params is envmerge's input contract: a plain data bag the caller fully
// populates (the config package builds it from a manifest plus CLI overrides).
// envmerge knows nothing about envx.yaml, cobra, or precedence. Every field is
// optional and envmerge fills in terminal defaults itself. The resolved knobs
// live in Settings, the value form envmerge merges.
type Params struct {
	// Includes is one project's ordered namespace chain, as absolute paths the
	// config package has already resolved against the workspace root.
	Includes []string
	// Environments lists the declared environments, used to validate the target.
	Environments []string
	// Settings holds the fully-resolved env-resolution knobs the merge reads.
	Settings Settings
}

// -------------------------------------------------------------------------------------

// Settings holds the fully-resolved env-resolution knobs envmerge consumes — a
// plain value struct with no knowledge of CLI precedence or YAML. Zero values
// are valid: an empty Env falls back to the first declared environment and the
// bool/string knobs default to off. The config package produces it by layering
// the declared schema.Settings over CLI input. It is the effective counterpart
// to the declared schema.Settings.
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

	// Validate the resolved environment against the declared set.
	if !slices.Contains(p.Environments, p.Settings.Env) {
		return fmt.Errorf(
			"environment %q is not declared in the manifest (available: %v)",
			p.Settings.Env, p.Environments,
		)
	}

	// No error means Params are normalized and valid.
	return nil
}
