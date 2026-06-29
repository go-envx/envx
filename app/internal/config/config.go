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
// envLookup is the signature for environment-variable lookup. It defaults to
// os.LookupEnv but can be replaced in tests.
type envLookup func(key string) (string, bool)

// -------------------------------------------------------------------------------------
// Input is the raw user input one action gathers at the frontend edge (a cobra
// command today, an HTTP handler tomorrow). Each setting is optional: a non-nil
// value means the user provided it explicitly and it wins the precedence chain,
// while nil means "fall through" to the ENVX_* var and then the manifest layers.
// ConfigPath selects the manifest. Resolve turns it into a *Resolved.
type Input struct {
	// ConfigPath selects the manifest file; nil or empty triggers auto-discovery.
	ConfigPath *string
	// Env is the explicitly requested target environment.
	Env *string
	// Strict, when set, requires every overlay file in the chain to exist.
	Strict *bool
	// Prefix is the explicitly requested key prefix.
	Prefix *string
	// Suffix is the explicitly requested key suffix.
	Suffix *string
	// NamespacePrefix, when set, prefixes each key with its namespace.
	NamespacePrefix *bool
	// Overload, when set, lets file values win over existing OS env vars.
	Overload *bool
}

// -------------------------------------------------------------------------------------
// Resolved is the aggregate config produces from one manifest load and one
// resolution pass. Actions read the fields they need and ignore the rest;
// OverlayPath derives the set action's target file from the same result.
type Resolved struct {
	// Envmerge is the resolved envmerge input for actions that merge an environment.
	Envmerge envmerge.Params
	// Overload is the resolved runner knob (flag > ENVX_OVERLOAD > project > global).
	Overload bool

	// manifest and dir are retained so OverlayPath can validate and join a target
	// without re-loading the manifest.
	manifest *schema.Manifest
	dir      string
}

// -------------------------------------------------------------------------------------
// Resolve loads the manifest (honoring --config, then ENVX_CONFIG, then a walk-up
// search) and meshes it with the input's values and ENVX_* vars into a single
// *Resolved, applying the precedence explicit > ENVX_* > project > global. An
// empty project resolves the global context only (no project layer, no includes,
// and no "project not found" error) for actions that never merge a project.
// Terminal fallbacks (e.g. the default environment) are applied downstream, so an
// unset env stays empty here.
func Resolve(in *Input, project string) (*Resolved, error) {
	m, manifestFile, err := manifest.Load(manifestPath(in))
	if err != nil {
		return nil, err
	}
	return resolveManifest(m, filepath.Dir(manifestFile), in, project)
}

// -------------------------------------------------------------------------------------
// resolveManifest applies the precedence layering against an already-loaded
// manifest. It is split from Resolve so the precedence chain stays unit-testable
// with an in-memory manifest. An empty project contributes no project layer and
// no includes.
func resolveManifest(
	m *schema.Manifest, dir string, in *Input, project string,
) (*Resolved, error) {
	var (
		proj     schema.Settings
		includes []string
	)
	if project != "" {
		p, ok := m.LookupProject(project)
		if !ok {
			return nil, fmt.Errorf("project %q not found in manifest", project)
		}
		proj = p.Settings
		includes = make([]string, len(p.Includes))
		for i, inc := range p.Includes {
			includes[i] = filepath.Join(dir, inc)
		}
	}

	r := newResolver()
	settings := envmerge.Settings{
		Env: r.String(
			&schema.Env, in.Env, proj.Env, m.Settings.Env,
		),
		Strict: r.Bool(
			&schema.Strict, in.Strict, proj.Strict, m.Settings.Strict,
		),
		Prefix: r.String(
			&schema.Prefix, in.Prefix, proj.Prefix, m.Settings.Prefix,
		),
		Suffix: r.String(
			&schema.Suffix, in.Suffix, proj.Suffix, m.Settings.Suffix,
		),
		NamespacePrefix: r.Bool(
			&schema.NamespacePrefix, in.NamespacePrefix,
			proj.NamespacePrefix, m.Settings.NamespacePrefix,
		),
	}
	return &Resolved{
		Envmerge: envmerge.Params{
			Includes:     includes,
			Environments: m.Environments,
			Settings:     settings,
		},
		Overload: r.Bool(
			&schema.Overload, in.Overload, proj.Overload, m.Settings.Overload,
		),
		manifest: m,
		dir:      dir,
	}, nil
}

// -------------------------------------------------------------------------------------
// OverlayPath resolves the absolute path of the overlay file the set action
// writes: <dir>/<include>.<env>.yaml. The environment is the resolved Env with
// the terminal first-declared fallback applied, validated against the declared
// set, and includePath is validated against the manifest before being joined
// against the workspace directory. It targets a single overlay file without
// merging an environment, so it never builds an envmerge result.
func (r *Resolved) OverlayPath(includePath string) (string, error) {
	env := r.Envmerge.Settings.Env
	if env == "" {
		env = r.manifest.DefaultEnvironment()
	}
	if !r.manifest.HasEnvironment(env) {
		return "", fmt.Errorf(
			"environment %q is not declared in the manifest (available: %v)",
			env, r.manifest.Environments,
		)
	}
	if !r.manifest.HasInclude(includePath) {
		return "", fmt.Errorf("include %q not found in manifest", includePath)
	}
	return filepath.Join(r.dir, includePath) + "." + env + ".yaml", nil
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
// resolver applies the precedence "explicit value > ENVX_* env var > layered
// defaults". It reads each setting's ENVX_* fallback straight from its
// schema.FlagSpec, so registration and resolution can never disagree about a
// name.
type resolver struct {
	LookupEnv envLookup
}

// -------------------------------------------------------------------------------------
// newResolver creates a resolver backed by os.LookupEnv.
func newResolver() *resolver {
	return &resolver{LookupEnv: os.LookupEnv}
}

// -------------------------------------------------------------------------------------
// String resolves a string setting: the explicit value wins when present, then
// the ENVX_* var, then the first non-empty layer (e.g. project then global
// default), and finally "".
func (r *resolver) String(
	s *schema.FlagSpec, explicit *string, layers ...string,
) string {
	if explicit != nil {
		return *explicit
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
// Bool resolves a boolean setting: the explicit value wins when present, then the
// ENVX_* var (parsed), then the first non-nil layer (e.g. project then global
// setting), and finally false.
func (r *resolver) Bool(
	s *schema.FlagSpec, explicit *bool, layers ...*bool,
) bool {
	if explicit != nil {
		return *explicit
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
