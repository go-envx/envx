package manifest

import (
	"os"
	"strconv"
)

// -------------------------------------------------------------------------------------
// EnvLookupFunc is the signature for environment variable lookup. Defaults to
// os.LookupEnv but can be replaced in tests.
type EnvLookupFunc func(key string) (string, bool)

// -------------------------------------------------------------------------------------
// FlagSet represents the subset of flag state needed for config resolution.
// This decouples config resolution from cobra's flag implementation.
type FlagSet interface {
	Changed(name string) bool
}

// -------------------------------------------------------------------------------------
// RawFlags holds the raw flag values as parsed by cobra. These are passed to
// Resolve along with the manifest and project to produce the final config.
type RawFlags struct {
	ConfigPath      string
	Environment     string
	Overload        bool
	Strict          bool
	Prefix          string
	Suffix          string
	NamespacePrefix bool
}

// -------------------------------------------------------------------------------------
// ResolvedConfig holds the fully resolved configuration after applying the
// precedence chain. This is the single source of truth passed to application
// logic — no further env var lookups or flag checks happen downstream.
type ResolvedConfig struct {
	Environment     string
	Overload        bool
	Strict          bool
	Prefix          string
	Suffix          string
	NamespacePrefix bool
}

// -------------------------------------------------------------------------------------
// Resolver resolves raw flag values into final configuration by applying the
// precedence chain. It holds a reference to the environment lookup function
// for testability.
type Resolver struct {
	LookupEnv EnvLookupFunc
}

// -------------------------------------------------------------------------------------
// NewResolver creates a Resolver using os.LookupEnv as the environment source.
func NewResolver() *Resolver {
	return &Resolver{LookupEnv: os.LookupEnv}
}

// -------------------------------------------------------------------------------------
// Resolve applies the full precedence chain to produce a ResolvedConfig.
// The flags parameter indicates which flags were explicitly set by the user
// (vs. retaining their default values).
func (r *Resolver) Resolve(
	flags *RawFlags,
	changed FlagSet,
	m *Manifest,
	proj *Project,
) ResolvedConfig {
	env := r.resolveString(
		"env", flags.Environment, changed,
		"ENVX_ENV",
		proj.Settings.DefaultEnvironment,
		m.Settings.DefaultEnvironment,
	)
	if env == "" {
		env = "development"
	}

	return ResolvedConfig{
		Environment: env,
		Overload: r.resolveBool(
			"overload", flags.Overload, changed,
			"ENVX_OVERLOAD",
			proj.Settings.Overload, m.Settings.Overload, false,
		),
		Strict: r.resolveBool(
			"strict", flags.Strict, changed,
			"ENVX_STRICT",
			proj.Settings.Strict, m.Settings.Strict, false,
		),
		Prefix: r.resolveString(
			"prefix", flags.Prefix, changed,
			"ENVX_PREFIX",
			proj.Settings.Prefix, m.Settings.Prefix,
		),
		Suffix: r.resolveString(
			"suffix", flags.Suffix, changed,
			"ENVX_SUFFIX",
			proj.Settings.Suffix, m.Settings.Suffix,
		),
		NamespacePrefix: r.resolveBool(
			"namespace-prefix", flags.NamespacePrefix, changed,
			"ENVX_NAMESPACE_PREFIX",
			proj.Settings.NamespacePrefix,
			m.Settings.NamespacePrefix, false,
		),
	}
}

// -------------------------------------------------------------------------------------
// resolveBool resolves a boolean setting using the precedence chain:
// Flag > Env Var > Project Settings > Global Settings > Default.
func (r *Resolver) resolveBool(
	flagName string, flagVal bool, changed FlagSet,
	envKey string, projVal, globalVal *bool, defaultVal bool,
) bool {
	if changed.Changed(flagName) {
		return flagVal
	}
	if envStr, ok := r.LookupEnv(envKey); ok {
		if b, err := strconv.ParseBool(envStr); err == nil {
			return b
		}
	}
	if projVal != nil {
		return *projVal
	}
	if globalVal != nil {
		return *globalVal
	}
	return defaultVal
}

// -------------------------------------------------------------------------------------
// resolveString resolves a string setting using the precedence chain:
// Flag > Env Var > Project Settings > Global Settings > Default (empty).
func (r *Resolver) resolveString(
	flagName, flagVal string, changed FlagSet,
	envKey, projVal, globalVal string,
) string {
	if changed.Changed(flagName) {
		return flagVal
	}
	if envStr, ok := r.LookupEnv(envKey); ok {
		return envStr
	}
	if projVal != "" {
		return projVal
	}
	return globalVal
}
