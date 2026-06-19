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
)

// -------------------------------------------------------------------------------------
// Request is the input contract for ResolveEnv: the loaded manifest, the target
// project, and the fully-resolved settings (including the target environment).
// All precedence resolution happens at the action edge, so the engine merely
// validates and merges.
type Request struct {
	Config   *config.Config
	Project  string
	Settings Settings
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
// ResolveEnv is the single entry point: it looks up the project, validates the
// resolved target environment, builds the namespace chain from the project's
// includes, deep-merges them, and returns an immutable Result. This is the only
// effectful engine call an action makes.
func ResolveEnv(req *Request) (*Result, error) {
	cfg := req.Config
	if cfg == nil {
		return nil, errors.New("engine: no manifest loaded")
	}

	match, ok := cfg.LookupProject(req.Project)
	if !ok {
		return nil, fmt.Errorf("project %q not found in manifest", req.Project)
	}
	proj := match.Project

	env := req.Settings.Env
	if !cfg.HasEnvironment(env) {
		return nil, fmt.Errorf(
			"environment %q is not declared in the manifest (available: %v)",
			env, cfg.Environments,
		)
	}

	opts := mergeOptions{
		strict:          req.Settings.Strict,
		prefix:          req.Settings.Prefix,
		suffix:          req.Settings.Suffix,
		namespacePrefix: req.Settings.NamespacePrefix,
	}
	return mergeNamespaces(buildNamespaces(cfg, &proj), env, opts)
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
