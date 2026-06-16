// Package merge handles loading YAML environment files, deep-merging them in
// declaration order, flattening nested keys to environment variables, and
// tracking provenance (which file each final value came from).
//
// This package is intentionally pure: it takes file paths in and returns a
// merged map + provenance out, with no side effects beyond file reads. This
// makes it the easiest and most critical module to test exhaustively.
package merge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// -------------------------------------------------------------------------------------
// Source records where a final env var value originated. File is the absolute
// path to the YAML file, and Key is the original nested key path before
// flattening (e.g. "postgres.username").
type Source struct {
	File string
	Key  string
}

// -------------------------------------------------------------------------------------
// Provenance tracks the winning source for an env var and all sources that
// were overridden during the merge process. This powers the "envx explain"
// command in Phase 2.
type Provenance struct {
	Winner     Source
	Overridden []Source
}

// -------------------------------------------------------------------------------------
// Result holds the final merged environment and full provenance information.
// Env contains the flattened KEY=VALUE pairs ready for injection into a child
// process. Provenance maps each key to its resolution history.
type Result struct {
	Env        map[string]string
	Provenance map[string]*Provenance
}

// -------------------------------------------------------------------------------------
// Options configures merge behavior.
type Options struct {
	// Strict requires environment overlay files to exist on disk. When false
	// (default), a missing overlay is silently skipped (base-only is valid).
	Strict bool

	// Prefix is prepended to all env var keys (e.g. "NUXT" → "NUXT_HOST").
	Prefix string

	// Suffix is appended to all env var keys (e.g. "_V2" → "HOST_V2").
	Suffix string

	// NamespacePrefix when true prefixes each env var with the namespace name
	// (e.g. namespace "postgres" → "POSTGRES_HOST"). Defaults to true when
	// the zero value is used; set explicitly to control behavior.
	NamespacePrefix bool
}

// -------------------------------------------------------------------------------------
// Namespace represents a single namespace to load. Each namespace resolves to
// up to two files:
//   - Base:    <Dir>/<Name>.yaml          (required — defines the namespace)
//   - Overlay: <Dir>/<Name>.<env>.yaml    (optional unless strict mode)
type Namespace struct {
	Dir  string // absolute path to the directory containing the namespace files
	Name string // base name of the namespace (e.g. "postgres", "api-core")
}

// -------------------------------------------------------------------------------------
// Resolve loads and merges a sequence of namespaces for the given environment.
// Namespaces are processed in declaration order; later namespaces override
// earlier ones (deterministic last-wins). Returns the fully merged environment
// and provenance tracking for every key.
func Resolve(
	namespaces []Namespace,
	environment string,
	opts Options,
) (*Result, error) {
	result := &Result{
		Env:        make(map[string]string),
		Provenance: make(map[string]*Provenance),
	}

	for _, ns := range namespaces {
		if err := loadNamespace(ns, environment, opts, result); err != nil {
			return nil, err
		}
	}

	// Apply global prefix/suffix to all keys.
	if opts.Prefix != "" || opts.Suffix != "" {
		result = applyPrefixSuffix(result, opts.Prefix, opts.Suffix)
	}

	return result, nil
}

// -------------------------------------------------------------------------------------
// loadNamespace loads a single namespace's base + optional overlay file,
// deep-merges them, flattens the result, and integrates it into the running
// Result (updating both Env and Provenance).
func loadNamespace(ns Namespace, env string, opts Options, result *Result) error {
	baseFile := filepath.Join(ns.Dir, ns.Name+".yaml")
	envFile := filepath.Join(ns.Dir, ns.Name+"."+env+".yaml")

	// Base file is always required — it defines that the namespace exists.
	baseMap, err := loadYAML(baseFile)
	if err != nil {
		return fmt.Errorf("loading base file %s: %w", baseFile, err)
	}

	// Environment overlay is optional unless strict mode.
	envMap, err := loadYAML(envFile)
	if err != nil {
		if opts.Strict || !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("loading environment file %s: %w", envFile, err)
		}
		envMap = nil
	}

	// Deep merge: env overlay on top of base.
	merged := deepMerge(baseMap, envMap)

	// Flatten to env vars and apply to result.
	flat, err := flatten(merged)
	if err != nil {
		return fmt.Errorf("namespace %s/%s: %w", ns.Dir, ns.Name, err)
	}

	// Determine source file for provenance (env file wins if key came from overlay).
	baseFlatKeys := flattenKeys(baseMap)
	for key, value := range flat {
		// Apply namespace prefix if enabled.
		finalKey := key
		if opts.NamespacePrefix {
			nsPrefix := toEnvKey(ns.Name)
			finalKey = nsPrefix + "_" + key
		}

		sourceFile := baseFile
		if envMap != nil {
			if _, inEnv := flattenSingle(envMap, key); inEnv {
				sourceFile = envFile
			}
		}

		src := Source{File: sourceFile, Key: baseFlatKeys[key]}
		if existing, ok := result.Provenance[finalKey]; ok {
			existing.Overridden = append(existing.Overridden, existing.Winner)
			existing.Winner = src
		} else {
			result.Provenance[finalKey] = &Provenance{Winner: src}
		}
		result.Env[finalKey] = value
	}

	return nil
}

// -------------------------------------------------------------------------------------
// loadYAML reads and unmarshals a YAML file into a generic map. Returns
// os.ErrNotExist (wrapped) when the file does not exist, allowing callers
// to distinguish "missing" from "malformed".
func loadYAML(path string) (map[string]any, error) {
	clean := filepath.Clean(path)
	//nolint:gosec // paths constructed from manifest config, not raw user input
	data, err := os.ReadFile(clean)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if m == nil {
		m = make(map[string]any)
	}
	return m, nil
}

// -------------------------------------------------------------------------------------
// deepMerge recursively merges src into dst using the following rules:
//   - Maps: merged recursively (keys from both sides are preserved).
//   - Scalars and lists: src replaces dst entirely.
//   - New keys in src are added to dst.
//
// Neither dst nor src is reused after the call; dst is modified in place.
func deepMerge(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any)
	}
	if src == nil {
		return dst
	}
	for k, srcVal := range src {
		dstVal, exists := dst[k]
		if !exists {
			dst[k] = srcVal
			continue
		}
		srcMap, srcIsMap := toMap(srcVal)
		dstMap, dstIsMap := toMap(dstVal)
		if srcIsMap && dstIsMap {
			dst[k] = deepMerge(dstMap, srcMap)
		} else {
			// Scalars and lists replace.
			dst[k] = srcVal
		}
	}
	return dst
}

// -------------------------------------------------------------------------------------
// toMap attempts to coerce a value into map[string]any. Handles both the
// standard map[string]any and the map[any]any variant that yaml.v3 may
// produce for certain inputs.
func toMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		// yaml.v3 can produce map[any]any in some cases.
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[fmt.Sprintf("%v", k)] = val
		}
		return result, true
	}
	return nil, false
}

// -------------------------------------------------------------------------------------
// flatten converts a nested map to flat KEY=VALUE pairs suitable for use as
// environment variables. Key paths are joined with "_" and uppercased:
//
//	postgres.username → POSTGRES_USERNAME
//
// Returns a hard error if two different nested paths produce the same flat key
// (e.g. "api_key" and "api.key" both → API_KEY). This prevents silent data loss.
func flatten(m map[string]any) (map[string]string, error) {
	result := make(map[string]string)
	origins := make(map[string]string)

	var walk func(prefix string, node map[string]any) error
	walk = func(prefix string, node map[string]any) error {
		for k, v := range node {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}

			flatKey := toEnvKey(path)

			if sub, isMap := toMap(v); isMap {
				if err := walk(path, sub); err != nil {
					return err
				}
			} else {
				if existing, collision := origins[flatKey]; collision {
					return fmt.Errorf("flatten collision: %q (from %q) and %q both produce key %q",
						path, existing, path, flatKey)
				}
				origins[flatKey] = path
				result[flatKey] = fmt.Sprintf("%v", v)
			}
		}
		return nil
	}

	if err := walk("", m); err != nil {
		return nil, err
	}
	return result, nil
}

// -------------------------------------------------------------------------------------
// toEnvKey converts a dotted nested path (e.g. "postgres.username") to the
// corresponding environment variable name (e.g. "POSTGRES_USERNAME").
// Dots and hyphens are replaced with underscores.
func toEnvKey(path string) string {
	s := strings.ReplaceAll(path, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return strings.ToUpper(s)
}

// -------------------------------------------------------------------------------------
// flattenKeys returns a lookup table mapping each flat env key back to its
// original dotted path. Used for provenance tracking.
func flattenKeys(m map[string]any) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string)
	var walk func(prefix string, node map[string]any)
	walk = func(prefix string, node map[string]any) {
		for k, v := range node {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			if sub, isMap := toMap(v); isMap {
				walk(path, sub)
			} else {
				result[toEnvKey(path)] = path
			}
		}
	}
	walk("", m)
	return result
}

// -------------------------------------------------------------------------------------
// flattenSingle checks whether a specific flat env key would be produced by
// flattening the given map. Used to determine whether an overlay file is the
// source of a particular key (for provenance attribution).
func flattenSingle(m map[string]any, targetKey string) (string, bool) {
	keys := flattenKeys(m)
	path, ok := keys[targetKey]
	return path, ok
}

// -------------------------------------------------------------------------------------
// applyPrefixSuffix transforms all keys in the result by prepending the prefix
// and appending the suffix. Both are uppercased and joined with "_".
func applyPrefixSuffix(r *Result, prefix, suffix string) *Result {
	newEnv := make(map[string]string, len(r.Env))
	newProv := make(map[string]*Provenance, len(r.Provenance))

	prefix = strings.ToUpper(strings.TrimRight(prefix, "_"))
	suffix = strings.ToUpper(strings.TrimLeft(suffix, "_"))

	for key, value := range r.Env {
		newKey := transformKey(key, prefix, suffix)
		newEnv[newKey] = value
	}
	for key, prov := range r.Provenance {
		newKey := transformKey(key, prefix, suffix)
		newProv[newKey] = prov
	}

	return &Result{Env: newEnv, Provenance: newProv}
}

// -------------------------------------------------------------------------------------
// transformKey applies prefix and suffix to a single key.
func transformKey(key, prefix, suffix string) string {
	if prefix != "" {
		key = prefix + "_" + key
	}
	if suffix != "" {
		key = key + "_" + suffix
	}
	return key
}
