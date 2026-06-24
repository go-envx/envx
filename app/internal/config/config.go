package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/manifest"
	"github.com/go-envx/envx/app/internal/schema"
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
// persistent --config path, the flag-bound envmerge settings, and the changed-flag
// handle that drives precedence. Resolve turns it into an *envmerge.Config.
type Input struct {
	ConfigPath *string
	Settings   envmerge.Settings
	Changed    FlagSet
}

// -------------------------------------------------------------------------------------
// Resolve loads the manifest (honoring --config, then ENVX_CONFIG, then a walk-up
// search) and meshes it with the input's flag values and ENVX_* vars into the
// *envmerge.Config for one project, applying the precedence
// flag > ENVX_* > project setting > global setting. Terminal fallbacks (e.g. the
// default environment) are left to envmerge, so an unset env stays empty here.
// A missing project yields the canonical "project not found" error.
func Resolve(in *Input, project string) (*envmerge.Config, error) {
	m, manifestFile, err := manifest.Load(manifestPath(in))
	if err != nil {
		return nil, err
	}
	return resolveManifest(m, filepath.Dir(manifestFile), in, project)
}

// -------------------------------------------------------------------------------------
// resolveManifest applies the precedence layering against an already-loaded
// manifest. It is split from Resolve so the precedence chain stays unit-testable
// with an in-memory manifest.
func resolveManifest(
	m *schema.Manifest, dir string, in *Input, project string,
) (*envmerge.Config, error) {
	proj, ok := m.LookupProject(project)
	if !ok {
		return nil, fmt.Errorf("project %q not found in manifest", project)
	}

	r := NewResolver()
	resolved := envmerge.Settings{
		Env: r.String(
			&schema.Env, in.Changed, in.Settings.Env,
			proj.Settings.Env, m.Settings.Env,
		),
		Strict: r.Bool(
			&schema.Strict, in.Changed, in.Settings.Strict,
			proj.Settings.Strict, m.Settings.Strict,
		),
		Prefix: r.String(
			&schema.Prefix, in.Changed, in.Settings.Prefix,
			proj.Settings.Prefix, m.Settings.Prefix,
		),
		Suffix: r.String(
			&schema.Suffix, in.Changed, in.Settings.Suffix,
			proj.Settings.Suffix, m.Settings.Suffix,
		),
		NamespacePrefix: r.Bool(
			&schema.NamespacePrefix, in.Changed, in.Settings.NamespacePrefix,
			proj.Settings.NamespacePrefix, m.Settings.NamespacePrefix,
		),
	}
	return &envmerge.Config{
		Dir:          dir,
		Includes:     proj.Includes,
		Environments: m.Environments,
		Settings:     resolved,
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
	if v := os.Getenv(schema.Config.Env); v != "" {
		return v
	}
	return ""
}

// -------------------------------------------------------------------------------------
// ResolveEnv meshes only the target environment (flag > ENVX_ENV > manifest
// global env) for callers that have no project — notably the set action, which
// writes a single overlay file and never invokes envmerge. The terminal
// first-declared-environment fallback is left to the caller
// (schema.DefaultEnvironment).
func ResolveEnv(m *schema.Manifest, rawEnv string, changed FlagSet) string {
	return NewResolver().String(&schema.Env, changed, rawEnv, m.Settings.Env)
}

// -------------------------------------------------------------------------------------
// ResolveOverload meshes only the --overload toggle (flag > ENVX_OVERLOAD) for
// the run action. Overload is not an envmerge setting and carries no manifest
// layer, so it is resolved on its own rather than riding along in Resolve.
func ResolveOverload(rawOverload bool, changed FlagSet) bool {
	return NewResolver().Bool(&schema.Overload, changed, rawOverload)
}

// -------------------------------------------------------------------------------------
// ResolveOverlayPath loads the manifest and resolves the absolute path of the
// overlay file one set call writes: <dir>/<name>.<env>.yaml. The target
// environment is meshed (flag > ENVX_ENV > manifest global > first declared) and
// validated against the declared set, and includePath is joined against the
// workspace directory. It serves the set action, which mutates a single overlay
// file without merging an environment, so it never builds an envmerge result.
func ResolveOverlayPath(in *Input, includePath string) (string, error) {
	m, manifestFile, err := manifest.Load(manifestPath(in))
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(manifestFile)
	env := ResolveEnv(m, in.Settings.Env, in.Changed)
	if env == "" {
		env = m.DefaultEnvironment()
	}
	if !m.HasEnvironment(env) {
		return "", fmt.Errorf(
			"environment %q is not declared in the manifest (available: %v)",
			env, m.Environments,
		)
	}
	if !m.HasInclude(includePath) {
		return "", fmt.Errorf("include %q not found in manifest", includePath)
	}
	return filepath.Join(dir, includePath) + "." + env + ".yaml", nil
}

// -------------------------------------------------------------------------------------
// Resolver applies the precedence "explicit flag > ENVX_* env var > layered
// defaults". It reads each flag's name and ENVX_* fallback straight from its
// schema.FlagSpec, so registration and resolution can never disagree about a
// name.
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
	s *schema.FlagSpec, changed FlagSet, flagVal string, layers ...string,
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
	s *schema.FlagSpec, changed FlagSet, flagVal bool, layers ...*bool,
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
