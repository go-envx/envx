package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/flags"
	"github.com/go-envx/envx/apps/envx/internal/manifest"
)

// -------------------------------------------------------------------------------------
// EnvLookup is the signature for environment-variable lookup. It defaults to
// os.LookupEnv but can be replaced in tests.
type EnvLookup func(key string) (string, bool)

// -------------------------------------------------------------------------------------
// FlagSet is the slice of cobra's flag state the resolver depends on. Accepting
// this interface (rather than *pflag.FlagSet) keeps config free of any CLI
// framework.
type FlagSet interface {
	Changed(name string) bool
}

// -------------------------------------------------------------------------------------
// Input is the raw user input one action gathers at the cobra edge: the
// persistent --config path, the flag-bound engine settings, and the changed-flag
// handle that drives precedence. Resolve turns it into an *engine.Config.
type Input struct {
	ConfigPath *string
	Settings   engine.Settings
	Changed    FlagSet
}

// -------------------------------------------------------------------------------------
// Resolve loads the manifest (honoring --config, then ENVX_CONFIG, then a walk-up
// search) and meshes it with the input's flag values and ENVX_* vars into the
// *engine.Config for one project, applying the precedence
// flag > ENVX_* > project setting > global setting. Terminal fallbacks (e.g. the
// default environment) are left to the engine, so an unset env stays empty here.
// A missing project yields the canonical "project not found" error.
func Resolve(in *Input, project string) (*engine.Config, error) {
	m, err := manifest.New(manifestPath(in))
	if err != nil {
		return nil, err
	}
	return resolveManifest(m, in, project)
}

// -------------------------------------------------------------------------------------
// resolveManifest applies the precedence layering against an already-loaded
// manifest. It is split from Resolve so the precedence chain stays unit-testable
// with an in-memory manifest.
func resolveManifest(
	m *manifest.Manifest, in *Input, project string,
) (*engine.Config, error) {
	pm, ok := m.LookupProject(project)
	if !ok {
		return nil, fmt.Errorf("project %q not found in manifest", project)
	}
	proj := pm.Project

	r := NewResolver()
	settings := engine.Settings{
		Env: r.String(
			&flags.Env, in.Changed, in.Settings.Env,
			proj.Settings.Env, m.Settings.Env,
		),
		Strict: r.Bool(
			&flags.Strict, in.Changed, in.Settings.Strict,
			proj.Settings.Strict, m.Settings.Strict,
		),
		Prefix: r.String(
			&flags.Prefix, in.Changed, in.Settings.Prefix,
			proj.Settings.Prefix, m.Settings.Prefix,
		),
		Suffix: r.String(
			&flags.Suffix, in.Changed, in.Settings.Suffix,
			proj.Settings.Suffix, m.Settings.Suffix,
		),
		NamespacePrefix: r.Bool(
			&flags.NamespacePrefix, in.Changed, in.Settings.NamespacePrefix,
			proj.Settings.NamespacePrefix, m.Settings.NamespacePrefix,
		),
	}
	return &engine.Config{
		Dir:          m.Dir(),
		Includes:     proj.Includes,
		Environments: m.Environments,
		Settings:     settings,
	}, nil
}

// -------------------------------------------------------------------------------------
// manifestPath resolves where the manifest lives, honoring the precedence
// --config flag > ENVX_CONFIG env var > "" (an empty result lets the manifest
// package walk up from the working directory).
func manifestPath(in *Input) string {
	if in.ConfigPath != nil && *in.ConfigPath != "" {
		return *in.ConfigPath
	}
	if v := os.Getenv(flags.Config.Env); v != "" {
		return v
	}
	return ""
}

// -------------------------------------------------------------------------------------
// ResolveEnv meshes only the target environment (flag > ENVX_ENV > manifest
// global env) for callers that have no project — notably the set action, which
// writes a single overlay file and never invokes the engine. The terminal
// "development" fallback is left to the caller (engine.DefaultEnv).
func ResolveEnv(m *manifest.Manifest, rawEnv string, changed FlagSet) string {
	return NewResolver().String(&flags.Env, changed, rawEnv, m.Settings.Env)
}

// -------------------------------------------------------------------------------------
// ResolveOverload meshes only the --overload toggle (flag > ENVX_OVERLOAD) for
// the run action. Overload is not an engine setting and carries no manifest
// layer, so it is resolved on its own rather than riding along in Resolve.
func ResolveOverload(rawOverload bool, changed FlagSet) bool {
	return NewResolver().Bool(&flags.Overload, changed, rawOverload)
}

// -------------------------------------------------------------------------------------
// ResolveTarget loads the manifest and resolves the overlay one set call writes:
// the target environment (flag > ENVX_ENV > manifest global > engine.DefaultEnv),
// validated against the declared set, plus the directory and base name for
// includePath. It serves the set action, which mutates a single overlay file
// without merging an environment, so it never builds an engine result.
func ResolveTarget(in *Input, includePath string) (env, dir, name string, err error) {
	m, err := manifest.New(manifestPath(in))
	if err != nil {
		return "", "", "", err
	}
	env = ResolveEnv(m, in.Settings.Env, in.Changed)
	if env == "" {
		env = engine.DefaultEnv
	}
	if !m.HasEnvironment(env) {
		return "", "", "", fmt.Errorf(
			"environment %q is not declared in the manifest (available: %v)",
			env, m.Environments,
		)
	}
	dir, name, ok := m.LookupInclude(includePath)
	if !ok {
		return "", "", "", fmt.Errorf("include %q not found in manifest", includePath)
	}
	return env, dir, name, nil
}

// -------------------------------------------------------------------------------------
// Resolver applies the precedence "explicit flag > ENVX_* env var > layered
// defaults". It reads each flag's name and ENVX_* fallback straight from its
// flags.Spec, so registration and resolution can never disagree about a name.
type Resolver struct {
	LookupEnv EnvLookup
}

// -------------------------------------------------------------------------------------
// NewResolver creates a Resolver backed by os.LookupEnv.
func NewResolver() *Resolver {
	return &Resolver{LookupEnv: os.LookupEnv}
}

// -------------------------------------------------------------------------------------
// String resolves a string setting: the flag value wins when the user set it,
// then the ENVX_* var, then the first non-empty layer (e.g. project then global
// default), and finally "".
func (r *Resolver) String(
	s *flags.Spec, changed FlagSet, flagVal string, layers ...string,
) string {
	if changed != nil && changed.Changed(s.Name) {
		return flagVal
	}
	if s.Env != "" {
		if v, ok := r.LookupEnv(s.Env); ok {
			return v
		}
	}
	for _, layer := range layers {
		if layer != "" {
			return layer
		}
	}
	return ""
}

// -------------------------------------------------------------------------------------
// Bool resolves a boolean setting: the flag value wins when the user set it,
// then the ENVX_* var (parsed), then the first non-nil layer (e.g. project then
// global setting), and finally false.
func (r *Resolver) Bool(
	s *flags.Spec, changed FlagSet, flagVal bool, layers ...*bool,
) bool {
	if changed != nil && changed.Changed(s.Name) {
		return flagVal
	}
	if s.Env != "" {
		if v, ok := r.LookupEnv(s.Env); ok {
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
