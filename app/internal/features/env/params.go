package env

import (
	"maps"
	"slices"
)

// defaultDelimiter joins list-valued leaves when no delimiter is configured.
const defaultDelimiter = ","

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
	// Overload controls source selection against the OS environment: false
	// (default) lets an OS value override a namespace key, true lets the namespace
	// value win. It mirrors exactly the precedence a run child process sees.
	Overload bool
	// ReferencePattern overrides the {{VAR}} reference syntax with a custom regular
	// expression whose first capture group is the variable name; an empty value
	// keeps the built-in default. It is compiled and validated at Service
	// construction.
	ReferencePattern string
}

// ProjectConfig defines one declared project's namespace includes and setting
// options.
type ProjectConfig struct {
	Name     string
	Includes []string
	Settings Options
}

// Options provides optional resolution setting overrides for an operation or
// project.
type Options struct {
	Delimiter        *string
	Env              *string
	Overload         *bool
	Prefix           *string
	ReferencePattern *string
	RequireOverlays  *bool
	Suffix           *string
}

// normalizeParams applies envmerge's structural terminal defaults, copies the
// caller-owned slices so later mutation cannot change service behavior, and
// returns the normalized params. It does not validate the environment: each
// operation validates the environment it actually uses, so an irrelevant default
// cannot block an operation that overrides it.
//
//nolint:gocritic // Internal normalizer copies caller parameters.
func normalizeParams(params ServiceParams) (ServiceParams, error) {
	// Apply the default list delimiter when none was configured.
	if params.Settings.Delimiter == "" {
		params.Settings.Delimiter = defaultDelimiter
	}

	// Copy caller-owned slices and the OS snapshot so caller mutation cannot
	// change service behavior.
	params.Includes = slices.Clone(params.Includes)
	params.Environments = slices.Clone(params.Environments)
	params.OSEnvironment = maps.Clone(params.OSEnvironment)

	if params.Projects != nil {
		cloned := make(map[string]ProjectConfig, len(params.Projects))
		for k, v := range params.Projects {
			v.Includes = slices.Clone(v.Includes)
			cloned[k] = v
		}
		params.Projects = cloned
	}

	return params, nil
}
