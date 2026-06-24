package envmerge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/pkg/file"
)

// -------------------------------------------------------------------------------------
// Config is envmerge's input contract: a plain data bag the caller fully
// populates (the config package builds it from a manifest plus CLI overrides).
// envmerge knows nothing about envx.yaml, cobra, or precedence. Every field
// is optional and envmerge fills in terminal defaults itself. The resolved
// knobs live in Settings, the value form envmerge merges.
type Config struct {
	Dir          string   // workspace root; include paths resolve against it
	Includes     []string // one project's ordered namespace chain
	Environments []string // declared environments, for validating the target
	Settings     Settings
}

// -------------------------------------------------------------------------------------
// Settings holds the fully-resolved env-resolution knobs envmerge consumes — a
// plain value struct with no knowledge of CLI precedence or YAML. Zero values are
// valid: an empty Env falls back to the first declared environment and the
// bool/string knobs default to off. The config package produces it by layering the
// declared schema.Settings over CLI input. It is the effective counterpart to the
// declared schema.Settings.
type Settings struct {
	Env             string
	Strict          bool
	Prefix          string
	Suffix          string
	NamespacePrefix bool
}

// -------------------------------------------------------------------------------------
// Build is the single entry point: it applies the default environment,
// validates it against the declared set, builds the namespace chain from the
// include list, deep-merges them, and returns an immutable Result. It performs no
// precedence resolution and reads no files beyond the namespace overlays.
func Build(c *Config) (*Result, error) {
	if c == nil {
		return nil, errors.New("envmerge: nil config")
	}

	env := c.Settings.Env
	if env == "" && len(c.Environments) > 0 {
		env = c.Environments[0]
	}
	if !slices.Contains(c.Environments, env) {
		return nil, fmt.Errorf(
			"environment %q is not declared in the manifest (available: %v)",
			env, c.Environments,
		)
	}

	opts := mergeOptions{
		strict:          c.Settings.Strict,
		prefix:          c.Settings.Prefix,
		suffix:          c.Settings.Suffix,
		namespacePrefix: c.Settings.NamespacePrefix,
	}
	return mergeNamespaces(buildNamespaces(c.Dir, c.Includes), env, opts)
}

// -------------------------------------------------------------------------------------
// buildNamespaces resolves each include into an absolute namespace (directory +
// base name), preserving declaration order. Includes are relative to dir.
func buildNamespaces(dir string, includes []string) []namespace {
	out := make([]namespace, 0, len(includes))
	for _, inc := range includes {
		d := filepath.Join(dir, filepath.Dir(inc))
		out = append(out, namespace{dir: d, name: filepath.Base(inc)})
	}
	return out
}

// -------------------------------------------------------------------------------------
// namespace identifies one namespace to load. It resolves to up to two files:
//   - base:    <dir>/<name>.yaml       (required — defines the namespace)
//   - overlay: <dir>/<name>.<env>.yaml (optional unless strict mode)
type namespace struct {
	dir  string
	name string
}

// -------------------------------------------------------------------------------------
// mergeOptions configures the deep-merge: whether overlays are mandatory, the
// global key prefix/suffix, and whether each key is prefixed with its
// namespace.
type mergeOptions struct {
	strict          bool
	prefix          string
	suffix          string
	namespacePrefix bool
}

// -------------------------------------------------------------------------------------
// mergeNamespaces loads and merges a sequence of namespaces for the given
// environment. Namespaces are processed in declaration order (deterministic
// last-wins) and the merged, flattened values plus per-key origins are wrapped
// in an immutable Result.
func mergeNamespaces(
	namespaces []namespace, environment string, opts mergeOptions,
) (*Result, error) {
	values := make(map[string]string)
	origins := make(map[string]Origin)

	for _, ns := range namespaces {
		if err := loadNamespace(ns, environment, opts, values, origins); err != nil {
			return nil, err
		}
	}

	if opts.prefix != "" || opts.suffix != "" {
		values, origins = applyPrefixSuffix(values, origins, opts.prefix, opts.suffix)
	}

	return &Result{values: values, origins: origins}, nil
}

// -------------------------------------------------------------------------------------
// loadNamespace loads one namespace's base and optional overlay file,
// deep-merges them, flattens the result to env-var keys, and integrates it into
// the running values/origins maps.
func loadNamespace(
	ns namespace, env string, opts mergeOptions,
	values map[string]string, origins map[string]Origin,
) error {
	baseFile := filepath.Join(ns.dir, ns.name+".yaml")
	envFile := filepath.Join(ns.dir, ns.name+"."+env+".yaml")

	baseMap, err := loadYAML(baseFile)
	if err != nil {
		return fmt.Errorf("loading base file %s: %w", baseFile, err)
	}

	envMap, err := loadYAML(envFile)
	if err != nil {
		if opts.strict || !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("loading environment file %s: %w", envFile, err)
		}
		envMap = nil
	}

	merged := deepMerge(baseMap, envMap)

	flat, err := flatten(merged)
	if err != nil {
		return fmt.Errorf("namespace %s/%s: %w", ns.dir, ns.name, err)
	}

	baseFlatKeys := flattenKeys(baseMap)
	for key, value := range flat {
		finalKey := key
		if opts.namespacePrefix {
			finalKey = toEnvKey(ns.name) + "_" + key
		}

		sourceFile := baseFile
		if envMap != nil {
			if _, inEnv := flattenSingle(envMap, key); inEnv {
				sourceFile = envFile
			}
		}

		src := Source{File: sourceFile, Key: baseFlatKeys[key]}
		if existing, ok := origins[finalKey]; ok {
			existing.Shadowed = append(existing.Shadowed, existing.Winner)
			existing.Winner = src
			origins[finalKey] = existing
		} else {
			origins[finalKey] = Origin{Winner: src}
		}
		values[finalKey] = value
	}

	return nil
}

// -------------------------------------------------------------------------------------
// loadYAML reads and unmarshals a YAML file into a generic map. It returns a
// wrapped os.ErrNotExist when the file is missing so callers can distinguish
// "missing" from "malformed".
func loadYAML(path string) (map[string]any, error) {
	data, err := file.Read(path)
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
// deepMerge recursively merges src into dst: maps are merged key-by-key, while
// scalars and lists from src replace those in dst. dst is modified in place and
// returned.
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
			dst[k] = srcVal
		}
	}
	return dst
}

// -------------------------------------------------------------------------------------
// toMap coerces a value into map[string]any, handling both the standard form
// and the map[any]any variant that yaml.v3 can produce.
func toMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[fmt.Sprintf("%v", k)] = val
		}
		return result, true
	}
	return nil, false
}

// -------------------------------------------------------------------------------------
// flatten converts a nested map to flat KEY=VALUE pairs suitable for env vars.
// Key paths are joined with "_" and uppercased (postgres.username ->
// POSTGRES_USERNAME). It returns an error when two distinct paths collapse to
// the same flat key, preventing silent data loss.
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
				continue
			}
			if existing, collision := origins[flatKey]; collision {
				return fmt.Errorf(
					"flatten collision: %q and %q both produce key %q",
					existing, path, flatKey,
				)
			}
			origins[flatKey] = path
			result[flatKey] = fmt.Sprintf("%v", v)
		}
		return nil
	}

	if err := walk("", m); err != nil {
		return nil, err
	}
	return result, nil
}

// -------------------------------------------------------------------------------------
// toEnvKey converts a dotted nested path (postgres.username) to its env-var
// name (POSTGRES_USERNAME). Dots and hyphens become underscores.
func toEnvKey(path string) string {
	s := strings.ReplaceAll(path, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return strings.ToUpper(s)
}

// -------------------------------------------------------------------------------------
// flattenKeys returns a lookup mapping each flat env key back to its original
// dotted path, used for origin tracking.
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
// flattenSingle reports whether targetKey would be produced by flattening m,
// returning the original dotted path when it would. Used to attribute a key to
// an overlay file.
func flattenSingle(m map[string]any, targetKey string) (string, bool) {
	path, ok := flattenKeys(m)[targetKey]
	return path, ok
}

// -------------------------------------------------------------------------------------
// applyPrefixSuffix returns fresh values/origins maps with every key prefixed
// and suffixed. Both affixes are uppercased and joined with "_".
func applyPrefixSuffix(
	srcValues map[string]string, srcOrigins map[string]Origin, prefix, suffix string,
) (values map[string]string, origins map[string]Origin) {
	prefix = strings.ToUpper(strings.TrimRight(prefix, "_"))
	suffix = strings.ToUpper(strings.TrimLeft(suffix, "_"))

	values = make(map[string]string, len(srcValues))
	for key, value := range srcValues {
		values[transformKey(key, prefix, suffix)] = value
	}
	origins = make(map[string]Origin, len(srcOrigins))
	for key, origin := range srcOrigins {
		origins[transformKey(key, prefix, suffix)] = origin
	}
	return values, origins
}

// -------------------------------------------------------------------------------------
// transformKey applies the prefix and suffix to a single key.
func transformKey(key, prefix, suffix string) string {
	if prefix != "" {
		key = prefix + "_" + key
	}
	if suffix != "" {
		key = key + "_" + suffix
	}
	return key
}
