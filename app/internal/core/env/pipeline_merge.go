package env

import (
	"fmt"
	"maps"
	"strings"
)

// namespace is one operation-local parsed base, reusable across environments.
type namespace struct {
	// include is the include path the base was loaded from.
	include string
	// baseFile is the base file's source path.
	baseFile string
	// baseFlat holds the base's flattened leaves.
	baseFlat map[string]leafValue
	// basePaths maps each base variable key to its dotted path.
	basePaths map[string]string
}

// mergedEnv is one environment merged from namespace files and host values,
// holding unresolved winners and provenance. It never contains resolved
// plaintext.
type mergedEnv struct {
	// values holds the unresolved winning leaf per variable key.
	values map[string]leafValue
	// origins records the winner and shadowed sources per key.
	origins map[string]Origin
	// declaredInBase marks keys some namespace base file declares.
	declaredInBase map[string]bool
	// overridden holds namespace leaves replaced by host values.
	overridden map[string]leafValue
}

// loadNamespaces reads and flattens every included base file once per
// operation. Read, YAML, and flatten failures are fatal.
func (s *Service) loadNamespaces(includes []string) ([]namespace, error) {
	loaded := make([]namespace, 0, len(includes))
	for _, include := range includes {
		data, err := s.deps.Namespaces.LoadBase(include)
		if err != nil {
			return nil, err
		}
		flat, paths, err := flatten(data.Data)
		if err != nil {
			return nil, fmt.Errorf("namespace %s: %w", include, err)
		}
		loaded = append(loaded, namespace{
			include:   include,
			baseFile:  data.SourcePath,
			baseFlat:  flat,
			basePaths: paths,
		})
	}
	return loaded, nil
}

// merge layers each namespace's overlay for op's environment over its base in
// declaration order (last wins), then applies the key affixes.
func (s *Service) merge(namespaces []namespace, op operation) (*mergedEnv, error) {
	merged := &mergedEnv{
		values:         make(map[string]leafValue),
		origins:        make(map[string]Origin),
		declaredInBase: make(map[string]bool),
		overridden:     make(map[string]leafValue),
	}
	for _, ns := range namespaces {
		overlay, err := s.loadOverlay(ns, op)
		if err != nil {
			return nil, err
		}
		if err := merged.layer(ns, overlay); err != nil {
			return nil, err
		}
	}
	merged.applyAffixes(op.settings.prefix, op.settings.suffix)
	return merged, nil
}

// loadOverlay reads ns's overlay for op's environment. A missing overlay has
// nil data unless overlays are required.
func (s *Service) loadOverlay(ns namespace, op operation) (NamespaceData, error) {
	overlay, found, err := s.deps.Namespaces.LoadOverlay(ns.include, op.environment)
	if err != nil {
		return NamespaceData{}, err
	}
	if found {
		return overlay, nil
	}
	if op.settings.requireOverlays {
		source := overlay.SourcePath
		if source == "" {
			source = ns.include + "." + op.environment + ".yaml"
		}
		return NamespaceData{}, fmt.Errorf(
			"loading environment file %s: %w", source, ErrOverlayNotFound,
		)
	}
	overlay.Data = nil
	return overlay, nil
}

// layer folds one namespace's overlay over its base into m.
func (m *mergedEnv) layer(ns namespace, overlay NamespaceData) error {
	// Flatten the overlay separately so nested and flat spellings of one key
	// override each other across files while still colliding within a file.
	overlayFlat, overlayPaths, err := flatten(overlay.Data)
	if err != nil {
		return fmt.Errorf("namespace %s: %w", ns.include, err)
	}

	flat := make(map[string]leafValue, len(ns.baseFlat)+len(overlayFlat))
	maps.Copy(flat, ns.baseFlat)
	maps.Copy(flat, overlayFlat)

	for key, leaf := range flat {
		var sources []Source
		if path, ok := ns.basePaths[key]; ok {
			sources = append(sources, Source{Path: ns.baseFile, Key: path})
			m.declaredInBase[key] = true
		}
		if path, ok := overlayPaths[key]; ok {
			sources = append(sources, Source{Path: overlay.SourcePath, Key: path})
		}
		m.integrate(key, sources)
		m.values[key] = leaf
	}
	return nil
}

// integrate folds one namespace's ordered sources for key into its origin. A
// previous origin is demoted, oldest first, and the last source wins.
func (m *mergedEnv) integrate(key string, sources []Source) {
	var shadowed []Source
	if existing, ok := m.origins[key]; ok {
		shadowed = append(shadowed, existing.Shadowed...)
		shadowed = append(shadowed, existing.Winner)
	}
	shadowed = append(shadowed, sources[:len(sources)-1]...)
	m.origins[key] = Origin{Winner: sources[len(sources)-1], Shadowed: shadowed}
}

// applyAffixes rewrites every key with the uppercased prefix and suffix.
func (m *mergedEnv) applyAffixes(prefix, suffix string) {
	if prefix == "" && suffix == "" {
		return
	}
	prefix = strings.ToUpper(strings.TrimRight(prefix, "_"))
	suffix = strings.ToUpper(strings.TrimLeft(suffix, "_"))
	rename := func(key string) string {
		if prefix != "" {
			key = prefix + "_" + key
		}
		if suffix != "" {
			key += "_" + suffix
		}
		return key
	}

	values := make(map[string]leafValue, len(m.values))
	for key, leaf := range m.values {
		values[rename(key)] = leaf
	}
	origins := make(map[string]Origin, len(m.origins))
	for key, origin := range m.origins {
		origins[rename(key)] = origin
	}
	declared := make(map[string]bool, len(m.declaredInBase))
	for key := range m.declaredInBase {
		declared[rename(key)] = true
	}
	m.values, m.origins, m.declaredInBase = values, origins, declared
}

// applyHost selects between namespace and op's host values. Without overload a
// host value wins and the namespace source is shadowed; with overload the
// namespace wins and the host source is shadowed. union adds host-only keys so
// a materialized child sees the complete environment.
func (m *mergedEnv) applyHost(op operation, union bool) {
	overload := op.settings.overload
	for key, text := range op.host.All() {
		hostSource := Source{Key: key, Host: true}
		prev, inNamespace := m.origins[key]
		switch {
		case inNamespace && !overload:
			shadowed := append(append([]Source{}, prev.Shadowed...), prev.Winner)
			m.overridden[key] = m.values[key]
			m.values[key] = m.hostLeaf(text)
			m.origins[key] = Origin{Winner: hostSource, Shadowed: shadowed}
		case inNamespace:
			shadowed := append(append([]Source{}, prev.Shadowed...), hostSource)
			m.origins[key] = Origin{Winner: prev.Winner, Shadowed: shadowed}
		case union:
			m.values[key] = m.hostLeaf(text)
			m.origins[key] = Origin{Winner: hostSource}
		}
	}
}

// opaque reports whether key's winning value is a host value.
func (m *mergedEnv) opaque(key string) bool {
	return m.values[key].opaque
}

// symbols is a lazy symbol table resolving and rendering winners on demand, so
// unreferenced failures never block a read.
func (m *mergedEnv) symbols(resolver ValueResolver, op operation) symbolTable {
	return symbolTable{
		declared: func(name string) bool { _, ok := m.values[name]; return ok },
		opaque:   m.opaque,
		value: func(name string) (string, error) {
			resolved, err := resolveLeaf(m.values[name], resolver, op.environment)
			if err != nil {
				return "", err
			}
			return renderLeaf(resolved, m.origins[name].Winner.Key, op.settings.delimiter)
		},
	}
}

// hostLeaf wraps a host value as an opaque scalar.
func (m *mergedEnv) hostLeaf(text string) leafValue {
	return leafValue{items: []string{text}, opaque: true}
}
