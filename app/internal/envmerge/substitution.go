package envmerge

import (
	"fmt"
	"sort"

	"github.com/go-envx/envx/app/internal/features/env/syntax"
)

// getenv returns a getenv seam backed by the injected OS-environment snapshot, so
// a reference resolves against the same environment used for source selection. A
// nil snapshot is an empty environment.
func (m *Manager) getenv() func(name string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := m.params.OSEnvironment[name]
		return value, ok
	}
}

// mapSymbols builds a syntax.SymbolTable over a fully resolved value map keyed
// the same as its origins.
func mapSymbols(
	values map[string]string, origins map[string]Origin,
) syntax.SymbolTable {
	return syntax.SymbolTable{
		Declared: func(name string) bool { _, ok := values[name]; return ok },
		Opaque: func(name string) bool {
			return origins[name].Winner.File == osSource
		},
		Value: func(name string) (string, error) { return values[name], nil },
	}
}

// newSubstituter builds a syntax.Substituter over the provided symbol table,
// wired to the manager's grammar, OS environment getter, and overload setting.
func (m *Manager) newSubstituter(symbols syntax.SymbolTable) *syntax.Substituter {
	return syntax.NewSubstituter(syntax.SubstituterParams{
		Grammar:  m.grammar,
		Symbols:  symbols,
		Getenv:   m.getenv(),
		Overload: m.params.Settings.Overload,
	})
}

// substituteAll composes every {{ }} reference over a fully resolved effective
// environment, transitively, returning the substituted values. An OS-sourced
// value is opaque and passes through untouched. A missing reference or a
// reference cycle is fatal and yields no partial map.
func (m *Manager) substituteAll(
	values map[string]string, origins map[string]Origin,
) (map[string]string, error) {
	engine := m.newSubstituter(mapSymbols(values, origins))
	out := make(map[string]string, len(values))
	for key := range values {
		composed, err := engine.Resolve(key)
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

// resolveEffectiveTolerant mirrors resolveEffective but downgrades every per-key
// failure to a returned warning and omits the failing key instead of aborting.
// materialize already omits a key whose secret fails to decrypt or whose list
// fails to render; this stage additionally omits any key whose {{ }} reference is
// missing or forms a cycle. Because each member of a cycle independently fails to
// resolve, the whole cycle is omitted. A failed key that the OS environment still
// defines falls back to that value so a broken file value never clobbers it.
func (m *Manager) resolveEffectiveTolerant(
	state *mergeState, resolver ValueResolver, environment string,
) (map[string]string, []error) {
	result := materialize(state, m.params.Settings, resolver, environment)

	// materialize leaves every failed key out of result.values; carry those
	// failures forward and add any substitution failure to them.
	failures := result.errs

	engine := m.newSubstituter(mapSymbols(result.values, result.origins))
	out := make(map[string]string, len(result.values))
	for key := range result.values {
		composed, err := engine.Resolve(key)
		if err != nil {
			failures[key] = err
			continue
		}
		out[key] = composed
	}

	return out, m.downgradeFailures(out, failures)
}

// downgradeFailures turns every unresolved key into a warning in sorted key order.
// When the OS environment still defines a failed key its value is restored into
// values, so a file value that cannot be produced never clobbers a value the OS
// or container already set — the case an --overload run hits when its file value
// is broken. A key with no OS fallback is left omitted.
func (m *Manager) downgradeFailures(
	values map[string]string, failures map[string]error,
) []error {
	if len(failures) == 0 {
		return nil
	}
	keys := make([]string, 0, len(failures))
	for key := range failures {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	warnings := make([]error, 0, len(keys))
	for _, key := range keys {
		if osValue, ok := m.params.OSEnvironment[key]; ok {
			values[key] = osValue
			warnings = append(warnings, fmt.Errorf(
				"%s: %w; keeping the value set in the environment", key, failures[key],
			))
			continue
		}
		warnings = append(warnings, fmt.Errorf("omitting %s: %w", key, failures[key]))
	}
	return warnings
}

// getSymbols builds a lazy syntax.SymbolTable over a merged state that resolves each
// referenced key's leaf on demand under the call's reveal policy. Only the keys
// reachable from the requested key are materialized, so an unrelated dangling
// reference never blocks the read while a dangling reference behind a referenced
// key surfaces its real error.
func (m *Manager) getSymbols(
	state *mergeState, resolver ValueResolver, environment string,
) syntax.SymbolTable {
	return syntax.SymbolTable{
		Declared: func(name string) bool { _, ok := state.values[name]; return ok },
		Opaque: func(name string) bool {
			return state.origins[name].Winner.File == osSource
		},
		Value: func(name string) (string, error) {
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
