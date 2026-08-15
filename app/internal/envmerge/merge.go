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

// loadedNamespace holds one operation-local parsed base so a multi-environment
// operation can reuse it across environments without re-reading the base file.
// Its snapshot is discarded when the operation returns.
type loadedNamespace struct {
	namespace
	// baseFile is the absolute path to the namespace's base file.
	baseFile string
	// baseFlat holds the base file's flattened, unresolved leaves.
	baseFlat map[string]leafValue
	// baseKeys maps each flat key back to its dotted path in the base file.
	baseKeys map[string]string
}

// mergeState holds unresolved winning leaves and provenance for one operation and
// environment. It never contains resolved plaintext.
type mergeState struct {
	// values holds unresolved scalar or list leaves by flattened env-var key.
	values map[string]leafValue
	// origins records the winner and shadowed sources for each key.
	origins map[string]Origin
}

// buildNamespaces resolves each include into a namespace (directory + base
// name), preserving declaration order. Includes are already absolute paths and no
// files are read.
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

// loadNamespaces reads and flattens every included base file into one
// operation-local snapshot. It performs the base-file I/O that a multi-environment
// operation would otherwise repeat per side. Malformed base YAML and flatten
// collisions are operation-fatal.
func (m *Manager) loadNamespaces() ([]loadedNamespace, error) {
	namespaces := buildNamespaces(m.params.Includes)
	loaded := make([]loadedNamespace, 0, len(namespaces))
	for _, ns := range namespaces {
		baseFile := filepath.Join(ns.dir, ns.name+".yaml")

		baseMap, err := loadYAML(baseFile)
		if err != nil {
			return nil, fmt.Errorf("loading base file %s: %w", baseFile, err)
		}
		baseFlat, err := flatten(baseMap)
		if err != nil {
			return nil, fmt.Errorf("namespace %s/%s: %w", ns.dir, ns.name, err)
		}

		loaded = append(loaded, loadedNamespace{
			namespace: ns,
			baseFile:  baseFile,
			baseFlat:  baseFlat,
			baseKeys:  flattenKeys(baseMap),
		})
	}
	return loaded, nil
}

// merge loads a fresh base snapshot for one environment and selects unresolved
// winners. An ordinary single-environment operation calls it; Diff reuses one
// snapshot across both sides via loadNamespaces and mergeLoaded directly.
func (m *Manager) merge(environment string) (*mergeState, error) {
	namespaces, err := m.loadNamespaces()
	if err != nil {
		return nil, err
	}
	return m.mergeLoaded(namespaces, environment)
}

// mergeLoaded applies one environment's overlays to an operation-local base
// snapshot, selecting winners and tracking provenance in declaration order
// (deterministic last-wins), then applies the global prefix and suffix.
func (m *Manager) mergeLoaded(
	namespaces []loadedNamespace, environment string,
) (*mergeState, error) {
	state := &mergeState{
		values:  make(map[string]leafValue),
		origins: make(map[string]Origin),
	}

	for _, ns := range namespaces {
		if err := loadNamespace(ns, environment, m.params.Settings, state); err != nil {
			return nil, err
		}
	}

	applyAffixes(state, m.params.Settings)
	return state, nil
}

// loadNamespace layers one namespace's optional environment overlay over its
// pre-loaded base, flattens the overlay, and integrates the unresolved result
// into the running values/origins maps. The namespace prefix is applied here,
// while the base name is available.
func loadNamespace(
	ns loadedNamespace, environment string, settings Settings, state *mergeState,
) error {
	envFile := filepath.Join(ns.dir, ns.name+"."+environment+".yaml")

	envMap, err := loadYAML(envFile)
	if err != nil {
		if settings.RequireOverlays || !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("loading environment file %s: %w", envFile, err)
		}
		envMap = nil
	}

	// Flatten the overlay independently, then layer it over the base. Equivalent
	// nested and flat spellings (log.level and log_level) collapse to the same env
	// key, so an overlay can override a base value written in the other style,
	// while flatten still rejects two spellings colliding within a single file.
	envFlat, err := flatten(envMap)
	if err != nil {
		return fmt.Errorf("namespace %s/%s: %w", ns.dir, ns.name, err)
	}

	flat := make(map[string]leafValue, len(ns.baseFlat)+len(envFlat))
	maps.Copy(flat, ns.baseFlat)
	maps.Copy(flat, envFlat)

	// Map each flat key back to its dotted overlay path once so the per-key loop
	// can attribute a value to the exact file(s) that defined it.
	envKeys := flattenKeys(envMap)
	for key, value := range flat {
		finalKey := key
		if settings.NamespacePrefix {
			finalKey = toEnvKey(ns.name) + "_" + key
		}

		sources := namespaceSources(key, ns.baseFile, envFile, ns.baseKeys, envKeys)
		integrateSources(state, finalKey, sources)
		state.values[finalKey] = value
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
func integrateSources(state *mergeState, key string, sources []Source) {
	var shadowed []Source
	if existing, ok := state.origins[key]; ok {
		shadowed = append(shadowed, existing.Shadowed...)
		shadowed = append(shadowed, existing.Winner)
	}
	shadowed = append(shadowed, sources[:len(sources)-1]...)

	state.origins[key] = Origin{
		Winner:   sources[len(sources)-1],
		Shadowed: shadowed,
	}
}

// applyAffixes rewrites the unresolved merge state's keys in place, prefixing and
// suffixing every key after namespace winners are selected. Both affixes are
// uppercased and joined with "_"; an empty prefix and suffix is a no-op.
func applyAffixes(state *mergeState, settings Settings) {
	if settings.Prefix == "" && settings.Suffix == "" {
		return
	}
	prefix := strings.ToUpper(strings.TrimRight(settings.Prefix, "_"))
	suffix := strings.ToUpper(strings.TrimLeft(settings.Suffix, "_"))

	values := make(map[string]leafValue, len(state.values))
	for key, value := range state.values {
		values[transformKey(key, prefix, suffix)] = value
	}

	origins := make(map[string]Origin, len(state.origins))
	for key, origin := range state.origins {
		origins[transformKey(key, prefix, suffix)] = origin
	}

	state.values = values
	state.origins = origins
}
