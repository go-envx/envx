package envmerge

import "sort"

// Change records one differing key and its rendered literal on each side.
type Change struct {
	// Key is the env-var key that differs between the two environments.
	Key string
	// Before is the literal winner under environment A (empty for additions).
	Before string
	// After is the literal winner under environment B (empty for removals).
	After string
}

// DiffResult is the sorted comparison between two complete environments. It
// compares rendered literal winners, so a changed reference is visible even when
// both references currently decrypt to the same plaintext.
type DiffResult struct {
	// EnvironmentA is the first ("before") environment name.
	EnvironmentA string
	// EnvironmentB is the second ("after") environment name.
	EnvironmentB string
	// Added lists keys present only in environment B, sorted by key.
	Added []Change
	// Removed lists keys present only in environment A, sorted by key.
	Removed []Change
	// Changed lists keys present in both with differing literals, sorted by key.
	Changed []Change
}

// Diff validates both environment names, loads and flattens every base namespace
// once for the call, applies each environment's overlays independently to that
// operation-local snapshot, and compares the two rendered literal maps. It never
// opens a resolver, reads the secrets store, or touches private keys, so dangling
// references and equal plaintext behind different references never hide a
// declaration change or block comparison. Namespace, YAML, flatten, and
// literal-render failures on either side are fatal and yield no partial result.
func (m *Manager) Diff(environmentA, environmentB string) (*DiffResult, error) {
	envA, err := m.normalizeEnvironment(environmentA)
	if err != nil {
		return nil, err
	}
	envB, err := m.normalizeEnvironment(environmentB)
	if err != nil {
		return nil, err
	}

	namespaces, err := m.loadNamespaces()
	if err != nil {
		return nil, err
	}

	stateA, err := m.mergeLoaded(namespaces, envA)
	if err != nil {
		return nil, err
	}
	stateB, err := m.mergeLoaded(namespaces, envB)
	if err != nil {
		return nil, err
	}

	delimiter := m.params.Settings.Delimiter
	literalsA, err := renderLiterals(stateA, delimiter)
	if err != nil {
		return nil, err
	}
	literalsB, err := renderLiterals(stateB, delimiter)
	if err != nil {
		return nil, err
	}

	return compare(envA, envB, literalsA, literalsB), nil
}

// renderLiterals renders every unresolved winner to its final env-var string with
// delimiter validation and no resolver, so an explicit or implicit reference is
// compared exactly as declared and a leading escape is not removed. A list item
// containing the delimiter fails without exposing the item value.
func renderLiterals(state *mergeState, delimiter string) (map[string]string, error) {
	out := make(map[string]string, len(state.values))
	for key, value := range state.values {
		rendered, err := renderLeafValue(value, state.origins[key].Winner.Key, delimiter)
		if err != nil {
			return nil, err
		}
		out[key] = rendered
	}
	return out, nil
}

// compare classifies the sorted union of two rendered literal environments into
// added, removed, and changed keys, each sorted by key.
func compare(environmentA, environmentB string, a, b map[string]string) *DiffResult {
	result := &DiffResult{EnvironmentA: environmentA, EnvironmentB: environmentB}
	for _, key := range unionKeys(a, b) {
		av, aok := a[key]
		bv, bok := b[key]
		switch {
		case aok && !bok:
			result.Removed = append(result.Removed, Change{Key: key, Before: av})
		case !aok && bok:
			result.Added = append(result.Added, Change{Key: key, After: bv})
		case av != bv:
			result.Changed = append(result.Changed, Change{Key: key, Before: av, After: bv})
		}
	}
	return result
}

// unionKeys returns the sorted union of two literal maps' keys.
func unionKeys(a, b map[string]string) []string {
	set := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		set[k] = struct{}{}
	}
	for k := range b {
		set[k] = struct{}{}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
