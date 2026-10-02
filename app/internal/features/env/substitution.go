package env

import (
	"fmt"
	"sort"

	"github.com/go-envx/envx/app/internal/features/env/syntax"
)

// getenv returns a getenv seam backed by the injected OS-environment snapshot, so
// a reference resolves against the same environment used for source selection. A
// nil snapshot is an empty environment.
func (s *Service) getenv() func(name string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := s.params.OSEnvironment[name]
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
func (s *Service) newSubstituter(symbols syntax.SymbolTable) *syntax.Substituter {
	return s.newSubstituterWith(symbols, s.grammar, s.params.Settings.Overload)
}

// newSubstituterWith builds a substituter using the specified grammar and
// overload setting.
func (s *Service) newSubstituterWith(
	symbols syntax.SymbolTable, grammar *syntax.Grammar, overload bool,
) *syntax.Substituter {
	return syntax.NewSubstituter(syntax.SubstituterParams{
		Grammar:  grammar,
		Symbols:  symbols,
		Getenv:   s.getenv(),
		Overload: overload,
	})
}

// substituteAll composes references using the specified grammar and overload.
func (s *Service) substituteAll(
	values map[string]string, origins map[string]Origin,
	grammar *syntax.Grammar, overload bool,
) (map[string]string, error) {
	engine := s.newSubstituterWith(mapSymbols(values, origins), grammar, overload)
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

// resolveEffective materializes and substitutes with specified settings and
// grammar.
func (s *Service) resolveEffective(
	state *mergeState, resolver ValueResolver, environment string,
	settings Settings, grammar *syntax.Grammar,
) (map[string]string, error) {
	result := materialize(state, settings, resolver, environment)
	if err := materializationError(result.errs); err != nil {
		return nil, err
	}
	return s.substituteAll(
		result.values, result.origins, grammar, settings.Overload,
	)
}

// resolveEffectiveTolerant resolves effective values tolerating errors using
// specified settings and grammar.
func (s *Service) resolveEffectiveTolerant(
	state *mergeState, resolver ValueResolver, environment string,
	settings Settings, grammar *syntax.Grammar,
) (map[string]string, []error) {
	result := materialize(state, settings, resolver, environment)

	// materialize leaves every failed key out of result.values; carry those
	// failures forward and add any substitution failure to them.
	failures := result.errs

	engine := s.newSubstituterWith(
		mapSymbols(result.values, result.origins), grammar, settings.Overload,
	)
	out := make(map[string]string, len(result.values))
	for key := range result.values {
		composed, err := engine.Resolve(key)
		if err != nil {
			failures[key] = err
			continue
		}
		out[key] = composed
	}

	return out, s.downgradeFailures(out, failures)
}

// downgradeFailures turns every unresolved key into a warning in sorted key order.
// When the OS environment still defines a failed key its value is restored into
// values, so a file value that cannot be produced never clobbers a value the OS
// or container already set — the case an --overload run hits when its file value
// is broken. A key with no OS fallback is left omitted.
func (s *Service) downgradeFailures(
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
		if osValue, ok := s.params.OSEnvironment[key]; ok {
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

// getSymbols builds a lazy syntax.SymbolTable over a merged state using the
// specified delimiter.
func (s *Service) getSymbols(
	state *mergeState, resolver ValueResolver, environment string, delimiter string,
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
				resolved, state.origins[name].Winner.Key, delimiter,
			)
		},
	}
}
