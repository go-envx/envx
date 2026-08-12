package envmerge

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// namespace identifies one namespace to load. It resolves to up to two files:
//   - base:    <dir>/<name>.yaml       (required — defines the namespace)
//   - overlay: <dir>/<name>.<env>.yaml (optional unless require_overlays)
type namespace struct {
	// dir is the absolute directory holding the namespace's YAML files.
	dir string
	// name is the namespace's base file name, without extension.
	name string
}

// Build is the single entry point: it normalizes the params (applying the
// default environment and validating it against the declared set), builds the
// namespace chain from the include list, deep-merges them, and returns an
// immutable Result. It reads no files beyond the namespace overlays.
func Build(p Params) (*Result, error) {
	// Normalize a local copy, leaving the caller's value untouched.
	if err := normalizeParams(&p); err != nil {
		return nil, err
	}

	// Build the namespace chain from the Includes list.
	namespaces := buildNamespaces(p.Includes)

	// Merge first, then dereference and render only the values that won.
	return mergeNamespaces(namespaces, p.Settings, p.ValueResolver)
}

// mergeState holds flattened values and origins while base, overlay, and
// namespace precedence selects the winners. Values remain unresolved here.
type mergeState struct {
	// values holds unresolved scalar or list leaves by flattened env-var key.
	values map[string]leafValue
	// origins records the winner and shadowed sources for each key.
	origins map[string]Origin
}

// resolveLeafValue dereferences each scalar item in one winning leaf value. List
// boundaries are preserved so rendering can validate and join the resolved
// items afterward. A nil resolver is an identity operation.
func resolveLeafValue(
	value leafValue, resolver ValueResolver, env string,
) (leafValue, error) {
	if resolver == nil {
		return value, nil
	}

	resolved := leafValue{
		items: make([]string, len(value.items)),
		list:  value.list,
	}
	for i, item := range value.items {
		result, err := resolver.Resolve(item, env)
		if err != nil {
			return leafValue{}, err
		}
		resolved.items[i] = result
	}
	return resolved, nil
}

// buildNamespaces resolves each include into a namespace (directory + base
// name), preserving declaration order. Includes are already absolute paths.
func buildNamespaces(includes []string) []namespace {
	out := make([]namespace, 0, len(includes))

	for _, inc := range includes {
		out = append(out, namespace{
			dir:  filepath.Dir(inc),
			name: filepath.Base(inc),
		})
	}

	return out
}

// mergeNamespaces loads and merges a sequence of namespaces using the resolved
// settings. Namespaces are processed in declaration order (deterministic
// last-wins). Only winning values are then dereferenced and rendered before the
// immutable Result is constructed.
func mergeNamespaces(
	namespaces []namespace, settings Settings, resolver ValueResolver,
) (*Result, error) {
	acc := &mergeState{
		values:  make(map[string]leafValue),
		origins: make(map[string]Origin),
	}

	for _, ns := range namespaces {
		if err := loadNamespace(ns, settings, acc); err != nil {
			return nil, err
		}
	}

	resolved := resolveMergedValues(acc, settings, resolver)
	if settings.Prefix != "" || settings.Suffix != "" {
		applyPrefixSuffix(resolved, settings.Prefix, settings.Suffix)
	}

	return &Result{resolved: *resolved}, nil
}

// resolveMergedValues dereferences and renders every winning value. A per-key
// resolution or render failure is recorded on the result instead of aborting,
// so a single-key consumer can ignore failures in unrelated keys while whole-
// environment consumers fail loudly via Result.Verify. Shadowed values are
// absent from mergeState.values and therefore never reach the resolver. Errors
// identify the winning env-var key without exposing its value.
func resolveMergedValues(
	acc *mergeState, settings Settings, resolver ValueResolver,
) *resolved {
	result := &resolved{
		values:  make(map[string]string, len(acc.values)),
		origins: acc.origins,
		errs:    make(map[string]error),
	}
	for key, value := range acc.values {
		resolvedValue, err := resolveLeafValue(value, resolver, settings.Env)
		if err != nil {
			result.errs[key] = err
			continue
		}
		path := acc.origins[key].Winner.Key
		rendered, err := renderLeafValue(resolvedValue, path, settings.Delimiter)
		if err != nil {
			result.errs[key] = err
			continue
		}
		result.values[key] = rendered
	}
	return result
}

// loadNamespace loads one namespace's base and optional overlay file, flattens
// each to env-var keys, layers the overlay over the base, and integrates the
// unresolved result into the running values/origins maps.
func loadNamespace(
	ns namespace, settings Settings, acc *mergeState,
) error {
	baseFile := filepath.Join(ns.dir, ns.name+".yaml")
	envFile := filepath.Join(ns.dir, ns.name+"."+settings.Env+".yaml")

	baseMap, err := loadYAML(baseFile)
	if err != nil {
		return fmt.Errorf("loading base file %s: %w", baseFile, err)
	}

	envMap, err := loadYAML(envFile)
	if err != nil {
		if settings.RequireOverlays || !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("loading environment file %s: %w", envFile, err)
		}
		envMap = nil
	}

	// Flatten each file to env-var keys independently, then layer the overlay
	// over the base. Equivalent nested and flat spellings (log.level and
	// log_level) collapse to the same env key, so an overlay can override a base
	// value written in the other style, while flatten still rejects two spellings
	// colliding within a single file. Values remain unresolved until all winners
	// have been selected.
	baseFlat, err := flatten(baseMap)
	if err != nil {
		return fmt.Errorf("namespace %s/%s: %w", ns.dir, ns.name, err)
	}
	envFlat, err := flatten(envMap)
	if err != nil {
		return fmt.Errorf("namespace %s/%s: %w", ns.dir, ns.name, err)
	}

	flat := make(map[string]leafValue, len(baseFlat)+len(envFlat))
	maps.Copy(flat, baseFlat)
	maps.Copy(flat, envFlat)

	// Map each flat key back to its dotted path in the base file and the env
	// overlay separately, computed once so the per-key loop can attribute a
	// value to the exact file(s) that defined it.
	baseKeys := flattenKeys(baseMap)
	envKeys := flattenKeys(envMap)
	for key, value := range flat {
		finalKey := key
		if settings.NamespacePrefix {
			finalKey = toEnvKey(ns.name) + "_" + key
		}

		sources := namespaceSources(key, baseFile, envFile, baseKeys, envKeys)
		integrateSources(acc, finalKey, sources)
		acc.values[finalKey] = value
	}

	return nil
}

// namespaceSources returns the sources one namespace contributes for a single
// flattened key, in merge order: the base file first, then the environment
// overlay. A key defined in both files yields two sources so the overlay, coming
// last, shadows the base. The slice always holds at least one source, since the
// key was produced by flattening the merge of exactly these two files.
func namespaceSources(
	key, baseFile, envFile string,
	baseKeys, envKeys map[string]string,
) []Source {
	var sources []Source
	if path, ok := baseKeys[key]; ok {
		sources = append(sources, Source{File: baseFile, Key: path})
	}
	if path, ok := envKeys[key]; ok {
		sources = append(sources, Source{File: envFile, Key: path})
	}
	return sources
}

// integrateSources folds one namespace's ordered sources for a key into the
// running origins, preserving the full merge history. An origin already recorded
// by an earlier namespace is demoted into the shadowed chain — its prior shadows
// first, then its former winner — ahead of this namespace's own lower-priority
// sources. The last source, the highest priority, becomes the new winner, so
// Shadowed always reads oldest-to-newest and Winner is the surviving source.
func integrateSources(acc *mergeState, key string, sources []Source) {
	var shadowed []Source
	if existing, ok := acc.origins[key]; ok {
		shadowed = append(shadowed, existing.Shadowed...)
		shadowed = append(shadowed, existing.Winner)
	}
	shadowed = append(shadowed, sources[:len(sources)-1]...)

	acc.origins[key] = Origin{
		Winner:   sources[len(sources)-1],
		Shadowed: shadowed,
	}
}

// applyPrefixSuffix rewrites the resolved keys in place, prefixing and suffixing every
// key. Both affixes are uppercased and joined with "_".
func applyPrefixSuffix(acc *resolved, prefix, suffix string) {
	prefix = strings.ToUpper(strings.TrimRight(prefix, "_"))
	suffix = strings.ToUpper(strings.TrimLeft(suffix, "_"))

	values := make(map[string]string, len(acc.values))
	for key, value := range acc.values {
		values[transformKey(key, prefix, suffix)] = value
	}

	origins := make(map[string]Origin, len(acc.origins))
	for key, origin := range acc.origins {
		origins[transformKey(key, prefix, suffix)] = origin
	}

	errs := make(map[string]error, len(acc.errs))
	for key, err := range acc.errs {
		errs[transformKey(key, prefix, suffix)] = err
	}

	acc.values = values
	acc.origins = origins
	acc.errs = errs
}
