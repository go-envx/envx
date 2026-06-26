package envmerge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// -------------------------------------------------------------------------------------

// namespace identifies one namespace to load. It resolves to up to two files:
//   - base:    <dir>/<name>.yaml       (required — defines the namespace)
//   - overlay: <dir>/<name>.<env>.yaml (optional unless strict mode)
type namespace struct {
	// dir is the absolute directory holding the namespace's YAML files.
	dir string
	// name is the namespace's base file name, without extension.
	name string
}

// -------------------------------------------------------------------------------------

// Build is the single entry point: it normalizes the params (applying the
// default environment and validating it against the declared set), builds the
// namespace chain from the include list, deep-merges them, and returns an
// immutable Result. It performs no precedence resolution and reads no files
// beyond the namespace overlays.
func Build(p *Params) (*Result, error) {
	if p == nil {
		return nil, errors.New("envmerge: nil params")
	}

	// Normalize the Params.
	if err := normalizeParams(p); err != nil {
		return nil, err
	}

	// Build the namespace chain from the Includes list.
	namespaces := buildNamespaces(p.Includes)

	// Merge the namespaces into a single Result.
	return mergeNamespaces(namespaces, p.Settings)
}

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

// mergeNamespaces loads and merges a sequence of namespaces using the resolved
// settings. Namespaces are processed in declaration order (deterministic
// last-wins) and the merged, flattened values plus per-key origins are wrapped
// in an immutable Result.
func mergeNamespaces(namespaces []namespace, settings Settings) (*Result, error) {
	acc := &resolved{
		values:  make(map[string]string),
		origins: make(map[string]Origin),
	}

	for _, ns := range namespaces {
		if err := loadNamespace(ns, settings, acc); err != nil {
			return nil, err
		}
	}

	if settings.Prefix != "" || settings.Suffix != "" {
		applyPrefixSuffix(acc, settings.Prefix, settings.Suffix)
	}

	return &Result{resolved: *acc}, nil
}

// -------------------------------------------------------------------------------------

// loadNamespace loads one namespace's base and optional overlay file,
// deep-merges them, flattens the result to env-var keys, and integrates it into
// the running values/origins maps.
func loadNamespace(ns namespace, settings Settings, acc *resolved) error {
	baseFile := filepath.Join(ns.dir, ns.name+".yaml")
	envFile := filepath.Join(ns.dir, ns.name+"."+settings.Env+".yaml")

	baseMap, err := loadYAML(baseFile)
	if err != nil {
		return fmt.Errorf("loading base file %s: %w", baseFile, err)
	}

	envMap, err := loadYAML(envFile)
	if err != nil {
		if settings.Strict || !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("loading environment file %s: %w", envFile, err)
		}
		envMap = nil
	}

	merged := deepMerge(baseMap, envMap)

	flat, err := flatten(merged)
	if err != nil {
		return fmt.Errorf("namespace %s/%s: %w", ns.dir, ns.name, err)
	}

	flatKeys := flattenKeys(merged)
	for key, value := range flat {
		finalKey := key
		if settings.NamespacePrefix {
			finalKey = toEnvKey(ns.name) + "_" + key
		}

		sourceFile := baseFile
		if envMap != nil {
			if _, inEnv := flattenSingle(envMap, key); inEnv {
				sourceFile = envFile
			}
		}

		src := Source{File: sourceFile, Key: flatKeys[key]}
		if existing, ok := acc.origins[finalKey]; ok {
			existing.Shadowed = append(existing.Shadowed, existing.Winner)
			existing.Winner = src
			acc.origins[finalKey] = existing
		} else {
			acc.origins[finalKey] = Origin{Winner: src}
		}
		acc.values[finalKey] = value
	}

	return nil
}

// -------------------------------------------------------------------------------------

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

	acc.values = values
	acc.origins = origins
}
