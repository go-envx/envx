// Package config is the CLI resolution pipeline: it meshes command-line flag
// values, ENVX_* environment variables, and the loaded manifest into the
// *engine.Config the engine consumes. Loading the manifest belongs to the
// manifest package; applying terminal defaults belongs to the engine. This
// package owns only the precedence layering in between and imports no CLI
// framework, so it stays reusable by a non-cobra frontend.
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
// Resolve meshes the raw flag values, ENVX_* vars, and manifest layers into the
// *engine.Config for one project, applying the precedence
// flag > ENVX_* > project setting > global setting. Terminal fallbacks (e.g. the
// default environment) are left to the engine, so an unset env stays empty here.
// A missing project yields the canonical "project not found" error.
func Resolve(
	m *manifest.Manifest, project string, raw engine.Settings, changed FlagSet,
) (*engine.Config, error) {
	pm, ok := m.LookupProject(project)
	if !ok {
		return nil, fmt.Errorf("project %q not found in manifest", project)
	}
	proj := pm.Project

	r := NewResolver()
	settings := engine.Settings{
		Env: r.String(
			flags.Env, changed, raw.Env, proj.Settings.Env, m.Settings.Env,
		),
		Strict: r.Bool(
			flags.Strict, changed, raw.Strict,
			proj.Settings.Strict, m.Settings.Strict,
		),
		Prefix: r.String(
			flags.Prefix, changed, raw.Prefix,
			proj.Settings.Prefix, m.Settings.Prefix,
		),
		Suffix: r.String(
			flags.Suffix, changed, raw.Suffix,
			proj.Settings.Suffix, m.Settings.Suffix,
		),
		NamespacePrefix: r.Bool(
			flags.NamespacePrefix, changed, raw.NamespacePrefix,
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
// ResolveEnv meshes only the target environment (flag > ENVX_ENV > manifest
// global env) for callers that have no project — notably the set action, which
// writes a single overlay file and never invokes the engine. The terminal
// "development" fallback is left to the caller (engine.DefaultEnv).
func ResolveEnv(m *manifest.Manifest, rawEnv string, changed FlagSet) string {
	return NewResolver().String(flags.Env, changed, rawEnv, m.Settings.Env)
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
	s flags.Spec, changed FlagSet, flagVal string, layers ...string,
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
	s flags.Spec, changed FlagSet, flagVal bool, layers ...*bool,
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
