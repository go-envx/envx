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

// DiffResult is the sorted comparison between two complete environments. Masked,
// it compares rendered literal winners, so a changed reference is visible even
// when both references currently decrypt to the same plaintext; revealed, it
// compares fully resolved and substituted values on each side.
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

// DiffParams selects the two environments to compare and the reveal policy, so a
// diff mirrors get: masked it compares declarations, revealed it compares
// resolved values.
type DiffParams struct {
	// EnvironmentA is the first ("before") environment name.
	EnvironmentA string
	// EnvironmentB is the second ("after") environment name.
	EnvironmentB string
	// Reveal controls whether each side is resolved and substituted before the
	// comparison.
	Reveal bool
}

// Diff validates both environment names, loads and flattens every base namespace
// once for the call, and applies each environment's overlays independently to
// that operation-local snapshot. Masked, it compares the two rendered literal
// maps without opening a resolver, so dangling references and equal plaintext
// behind different references never hide a declaration change or block
// comparison. Revealed, it resolves and substitutes every value on each side, so
// a dangling reference or cycle on either side is fatal. Namespace, YAML,
// flatten, and render failures on either side are fatal and yield no partial
// result.
func (m *Manager) Diff(params DiffParams) (*DiffResult, error) {
	envA, err := m.normalizeEnvironment(params.EnvironmentA)
	if err != nil {
		return nil, err
	}
	envB, err := m.normalizeEnvironment(params.EnvironmentB)
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

	// Apply OS source selection to each side so an OS override is compared exactly
	// as run would see it; OS-only keys are excluded from the comparison.
	m.applyOSEnvironment(stateA, false)
	m.applyOSEnvironment(stateB, false)

	valuesA, valuesB, err := m.diffValues(params.Reveal, stateA, stateB, envA, envB)
	if err != nil {
		return nil, err
	}

	return compare(envA, envB, valuesA, valuesB), nil
}

// diffValues renders each side's values for comparison under the call's reveal
// policy: masked, each side is rendered to its declared literals without a
// resolver; revealed, each side is resolved and substituted through a shared
// revealing resolver.
func (m *Manager) diffValues(
	reveal bool, stateA, stateB *mergeState, envA, envB string,
) (mapA, mapB map[string]string, err error) {
	if !reveal {
		delimiter := m.params.Settings.Delimiter
		literalsA, lerr := renderLiterals(stateA, delimiter)
		if lerr != nil {
			return nil, nil, lerr
		}
		literalsB, lerr := renderLiterals(stateB, delimiter)
		if lerr != nil {
			return nil, nil, lerr
		}
		return literalsA, literalsB, nil
	}

	resolver, rerr := m.openResolver(true)
	if rerr != nil {
		return nil, nil, rerr
	}
	valuesA, rerr := m.resolveEffective(stateA, resolver, envA)
	if rerr != nil {
		return nil, nil, rerr
	}
	valuesB, rerr := m.resolveEffective(stateB, resolver, envB)
	if rerr != nil {
		return nil, nil, rerr
	}
	return valuesA, valuesB, nil
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
