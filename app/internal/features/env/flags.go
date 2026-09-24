package env

import (
	"os"
	"strconv"

	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/spf13/pflag"
)

// Environment synthesis and resolution flag definitions.
var (
	// Env selects the target environment.
	Env = flags.Spec[string]{
		Name:  "env",
		Short: "E",
		Env:   "ENVX_ENV",
		Usage: "target environment (defaults to first declared environment in envx.yaml)",
	}

	// RequireOverlays requires every environment overlay file in the namespace to exist.
	RequireOverlays = flags.Spec[bool]{
		Name:  "require-overlays",
		Env:   "ENVX_REQUIRE_OVERLAYS",
		Usage: "require all environment overlay files to exist",
	}

	// Prefix is prepended to every resolved env-var key.
	Prefix = flags.Spec[string]{
		Name:  "prefix",
		Env:   "ENVX_PREFIX",
		Usage: "prefix prepended to every key",
	}

	// Suffix is appended to every resolved env-var key.
	Suffix = flags.Spec[string]{
		Name:  "suffix",
		Env:   "ENVX_SUFFIX",
		Usage: "suffix appended to every key",
	}

	// Delimiter joins a list-valued leaf into a single env var.
	Delimiter = flags.Spec[string]{
		Name:  "delimiter",
		Env:   "ENVX_DELIMITER",
		Usage: `string used to join list values (default ",")`,
	}

	// Overload lets file values override existing OS env vars.
	Overload = flags.Spec[bool]{
		Name:  "overload",
		Env:   "ENVX_OVERLOAD",
		Usage: "file values override OS env vars",
	}

	// ReferencePattern overrides the {{VAR}} reference syntax with a regex.
	ReferencePattern = flags.Spec[string]{
		Name:  "reference-pattern",
		Env:   "ENVX_REFERENCE_PATTERN",
		Usage: "regex overriding the {{VAR}} reference syntax (group 1 is the name)",
	}

	// Reveal decrypts referenced secret values instead of masking them.
	Reveal = flags.Spec[bool]{
		Name:  "reveal",
		Usage: "decrypt secret references instead of masking them",
	}

	// Absolute renders source paths absolutely instead of relative to envx.yaml.
	Absolute = flags.Spec[bool]{
		Name:  "absolute",
		Usage: "show absolute source paths instead of paths relative to envx.yaml",
	}
)

// Option registers one resolution setting's flag on a flag set.
type Option func(*pflag.FlagSet)

// RegisterFlags applies each option to fs.
func RegisterFlags(fs *pflag.FlagSet, opts ...Option) {
	for _, opt := range opts {
		opt(fs)
	}
}

// WithEnv registers the --env flag on fs.
func WithEnv(fs *pflag.FlagSet) {
	flags.Bind(fs, new(string), &Env)
}

// WithRequireOverlays registers the --require-overlays flag on fs.
func WithRequireOverlays(fs *pflag.FlagSet) {
	flags.Bind(fs, new(bool), &RequireOverlays)
}

// WithPrefix registers the --prefix flag on fs.
func WithPrefix(fs *pflag.FlagSet) {
	flags.Bind(fs, new(string), &Prefix)
}

// WithSuffix registers the --suffix flag on fs.
func WithSuffix(fs *pflag.FlagSet) {
	flags.Bind(fs, new(string), &Suffix)
}

// WithDelimiter registers the --delimiter flag on fs.
func WithDelimiter(fs *pflag.FlagSet) {
	flags.Bind(fs, new(string), &Delimiter)
}

// WithOverload registers the --overload flag on fs.
func WithOverload(fs *pflag.FlagSet) {
	flags.Bind(fs, new(bool), &Overload)
}

// WithReferencePattern registers the --reference-pattern flag on fs.
func WithReferencePattern(fs *pflag.FlagSet) {
	flags.Bind(fs, new(string), &ReferencePattern)
}

// PrecedenceString resolves a string setting: the explicit value wins when present,
// then the ENVX_* var, then the first non-empty layer (e.g. project then
// global default), and finally "". Nil and empty layers are both skipped.
func PrecedenceString(
	s *flags.Spec[string],
	explicit *string,
	layers ...*string,
) string {
	if explicit != nil {
		return *explicit
	}
	if s.Env != "" {
		if v, ok := os.LookupEnv(s.Env); ok {
			return v
		}
	}
	for _, layer := range layers {
		if layer != nil && *layer != "" {
			return *layer
		}
	}
	return ""
}

// PrecedenceBool resolves a boolean setting: the explicit value wins when present,
// then the ENVX_* var (parsed), then the first non-nil layer (e.g. project then
// global setting), and finally false.
func PrecedenceBool(s *flags.Spec[bool], explicit *bool, layers ...*bool) bool {
	if explicit != nil {
		return *explicit
	}
	if s.Env != "" {
		if v, ok := os.LookupEnv(s.Env); ok {
			if b, err := strconv.ParseBool(v); err == nil {
				return b
			}
		}
	}
	for _, layer := range layers {
		if layer != nil {
			return *layer
		}
	}
	return false
}
