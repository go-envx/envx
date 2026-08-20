package envmerge

// getenv returns a getenv seam backed by the injected OS-environment snapshot, so
// a {{@VAR}} reference resolves against the same environment used for source
// selection. A nil snapshot is an empty environment.
func (m *Manager) getenv() func(name string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := m.params.OSEnvironment[name]
		return value, ok
	}
}

// substituteAll composes every {{ }} reference over a fully resolved effective
// environment, transitively, returning the substituted values. An OS-sourced
// value is opaque and passes through untouched. A missing reference or a
// reference cycle is fatal and yields no partial map.
func (m *Manager) substituteAll(
	values map[string]string, origins map[string]Origin,
) (map[string]string, error) {
	engine := newSymbolSubstituter(
		mapSymbols(values, origins), m.getenv(), m.params.Settings.Overload,
	)
	out := make(map[string]string, len(values))
	for key := range values {
		composed, err := engine.resolve(key)
		if err != nil {
			return nil, err
		}
		out[key] = composed
	}
	return out, nil
}

// resolveEffective materializes every winning value and then substitutes every
// {{ }} reference over the resulting effective environment. It is the reveal-path
// core shared by Materialize and a revealed Diff: both reveal every value, so a
// dangling reference or a cycle anywhere is fatal and no partial result escapes.
func (m *Manager) resolveEffective(
	state *mergeState, resolver ValueResolver, environment string,
) (map[string]string, error) {
	result := materialize(state, m.params.Settings, resolver, environment)
	if err := materializationError(result.errs); err != nil {
		return nil, err
	}
	return m.substituteAll(result.values, result.origins)
}

// getSymbols builds a lazy symbolTable over a merged state that resolves each
// referenced key's leaf on demand under the call's reveal policy. Only the keys
// reachable from the requested key are materialized, so an unrelated dangling
// reference never blocks the read while a dangling reference behind a referenced
// key surfaces its real error.
func (m *Manager) getSymbols(
	state *mergeState, resolver ValueResolver, environment string,
) symbolTable {
	return symbolTable{
		declared: func(name string) bool { _, ok := state.values[name]; return ok },
		opaque: func(name string) bool {
			return state.origins[name].Winner.File == osSource
		},
		value: func(name string) (string, error) {
			resolved, err := resolveLeaf(state.values[name], resolver, environment)
			if err != nil {
				return "", err
			}
			return renderLeafValue(
				resolved, state.origins[name].Winner.Key, m.params.Settings.Delimiter,
			)
		},
	}
}
