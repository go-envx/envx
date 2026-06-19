// Package engine owns environment resolution: project lookup, environment
// validation, namespace building, deep-merge, and origin tracking. The scattered
// "manifest -> namespace -> merge" pipeline lives entirely behind ResolveEnv,
// which returns one immutable Result. The merge internals are unexported
// (merge.go) because the engine is their only consumer — actions depend on the
// engine, never on merge directly.
package engine

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"sort"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/flags"
)

// -------------------------------------------------------------------------------------
// Request is the input contract for ResolveEnv. Environment is an optional
// per-call override (diff passes each side); when empty the engine derives the
// environment from the shared Global plus project/manifest defaults.
type Request struct {
	Global      config.Global
	Project     string
	Environment string
	Flags       Flags
	Changed     config.FlagSet
}

// -------------------------------------------------------------------------------------
// Result is the immutable outcome of resolution. Its maps are unexported;
// consumers read them through the methods below, which return copies or sorted
// views so the internals stay un-mutated.
type Result struct {
	values  map[string]string
	origins map[string]Origin
}

// -------------------------------------------------------------------------------------
// Source records where a resolved value came from: the absolute path to the
// YAML file and the original nested key before flattening (e.g.
// "postgres.password").
type Source struct {
	File string
	Key  string
}

// -------------------------------------------------------------------------------------
// Origin describes where a resolved value came from, plus every source it
// shadowed during the merge. It powers the explain action.
type Origin struct {
	Winner   Source
	Shadowed []Source
}

// -------------------------------------------------------------------------------------
// Get returns the value of a single key and whether it was present.
func (r *Result) Get(key string) (string, bool) {
	v, ok := r.values[key]
	return v, ok
}

// -------------------------------------------------------------------------------------
// All returns a copy of the full resolved environment, safe for the caller to
// mutate.
func (r *Result) All() map[string]string {
	out := make(map[string]string, len(r.values))
	maps.Copy(out, r.values)
	return out
}

// -------------------------------------------------------------------------------------
// Origin returns the resolution history for a key and whether it was present.
func (r *Result) Origin(key string) (Origin, bool) {
	o, ok := r.origins[key]
	return o, ok
}

// -------------------------------------------------------------------------------------
// Keys returns the resolved keys in sorted order.
func (r *Result) Keys() []string {
	keys := make([]string, 0, len(r.values))
	for k := range r.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// -------------------------------------------------------------------------------------
// ResolveEnv is the single entry point: it looks up the project, resolves and
// validates the target environment, resolves the merge settings, builds the
// namespace chain from the project's includes, deep-merges them, and returns an
// immutable Result. This is the only effectful engine call an action makes.
func ResolveEnv(req *Request) (*Result, error) {
	cfg := req.Global.Config
	if cfg == nil {
		return nil, errors.New("engine: no manifest loaded")
	}

	match, ok := cfg.LookupProject(req.Project)
	if !ok {
		return nil, fmt.Errorf("project %q not found in manifest", req.Project)
	}
	proj := match.Project

	resolver := config.NewResolver()
	env := resolveEnvironment(req, &proj, resolver)
	if !cfg.HasEnvironment(env) {
		return nil, fmt.Errorf(
			"environment %q is not declared in the manifest (available: %v)",
			env, cfg.Environments,
		)
	}

	opts := resolveOptions(req, &proj, cfg, resolver)
	return mergeNamespaces(buildNamespaces(cfg, &proj), env, opts)
}

// -------------------------------------------------------------------------------------
// resolveEnvironment applies the environment precedence (first non-empty wins):
// explicit per-call override > --env/ENVX_ENV > project default > the base
// Global.Environment (which already folds in the global default and the
// "development" fallback).
func resolveEnvironment(
	req *Request, proj *config.Project, resolver *config.Resolver,
) string {
	if req.Environment != "" {
		return req.Environment
	}

	explicit := req.Changed != nil && req.Changed.Changed(flags.Env.Name)
	if !explicit {
		if _, ok := resolver.LookupEnv(flags.Env.Env); ok {
			explicit = true
		}
	}
	if explicit {
		return req.Global.Environment
	}

	if proj.Settings.DefaultEnvironment != "" {
		return proj.Settings.DefaultEnvironment
	}
	return req.Global.Environment
}

// -------------------------------------------------------------------------------------
// resolveOptions resolves the merge settings (strict/prefix/suffix/namespace)
// using the precedence flag > ENVX_* > project setting > global setting > zero.
func resolveOptions(
	req *Request, proj *config.Project, cfg *config.Config,
	resolver *config.Resolver,
) mergeOptions {
	return mergeOptions{
		strict: resolver.Bool(
			flags.Strict, req.Changed, req.Flags.Strict,
			proj.Settings.Strict, cfg.Settings.Strict,
		),
		prefix: resolver.String(
			flags.Prefix, req.Changed, req.Flags.Prefix,
			proj.Settings.Prefix, cfg.Settings.Prefix,
		),
		suffix: resolver.String(
			flags.Suffix, req.Changed, req.Flags.Suffix,
			proj.Settings.Suffix, cfg.Settings.Suffix,
		),
		namespacePrefix: resolver.Bool(
			flags.NamespacePrefix, req.Changed, req.Flags.NamespacePrefix,
			proj.Settings.NamespacePrefix, cfg.Settings.NamespacePrefix,
		),
	}
}

// -------------------------------------------------------------------------------------
// buildNamespaces resolves each of a project's includes into an absolute
// namespace (directory + base name), preserving declaration order.
func buildNamespaces(cfg *config.Config, proj *config.Project) []namespace {
	out := make([]namespace, 0, len(proj.Includes))
	for _, inc := range proj.Includes {
		dir := filepath.Join(cfg.Dir(), filepath.Dir(inc))
		out = append(out, namespace{dir: dir, name: filepath.Base(inc)})
	}
	return out
}
