package envmerge

import (
	"errors"
	"fmt"
	"maps"
	"sort"
)

// Source records where a resolved value came from: the absolute path to the
// YAML file and the original nested key before flattening (e.g.
// "postgres.password").
type Source struct {
	// File is the absolute path to the YAML file the value came from.
	File string
	// Key is the original nested key before flattening (e.g. "postgres.password").
	Key string
}

// Origin describes where a resolved value came from, plus every source it
// shadowed during the merge.
type Origin struct {
	// Winner is the source whose value survived the merge.
	Winner Source
	// Shadowed lists the sources the winner overrode, in merge order.
	Shadowed []Source
}

// Environment is an immutable, complete set of materialized values and their
// provenance. A Manager returns one only when every winning value resolved, so a
// caller can never observe a partial or reference-carrying environment.
type Environment struct {
	// values holds the materialized env-var key/value pairs.
	values map[string]string
	// origins records, per key, the winning source and any it shadowed.
	origins map[string]Origin
}

// Get returns one materialized value and whether it exists.
func (e *Environment) Get(key string) (string, bool) {
	v, ok := e.values[key]
	return v, ok
}

// All returns a mutable copy of every materialized key/value pair, safe for the
// caller to mutate.
func (e *Environment) All() map[string]string {
	out := make(map[string]string, len(e.values))
	maps.Copy(out, e.values)
	return out
}

// Keys returns all materialized keys in sorted order.
func (e *Environment) Keys() []string {
	keys := make([]string, 0, len(e.values))
	for k := range e.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Origin returns one key's provenance and whether it exists.
func (e *Environment) Origin(key string) (Origin, bool) {
	o, ok := e.origins[key]
	return o, ok
}

// materializedState accumulates complete-resolution values and per-key failures
// inside one call. It is never embedded into a public result when errs is
// non-empty; partial values stay operation-local and are discarded on error.
type materializedState struct {
	// values holds successfully materialized env-var key/value pairs.
	values map[string]string
	// origins records, per key, the winning source and any it shadowed.
	origins map[string]Origin
	// errs records, per key, a resolution or render failure.
	errs map[string]error
}

// MaterializeParams selects the environment to materialize and the failure
// policy. It mirrors GetParams and DiffParams so every Manager operation is
// driven by one struct.
type MaterializeParams struct {
	// Environment overrides the configured default; an empty value uses it.
	Environment string
	// IgnoreErrors downgrades every per-key resolution failure to a warning and
	// omits the failing key instead of aborting, so the child process still starts.
	IgnoreErrors bool
}

// MaterializeResult is a complete materialized environment plus any downgraded
// warnings. In the default fail-closed mode Warnings is nil, because any failure
// aborts with an error instead; under IgnoreErrors it lists the omitted keys in
// sorted key order.
type MaterializeResult struct {
	// Environment is the complete materialized environment.
	Environment *Environment
	// Warnings lists the downgraded per-key failures under IgnoreErrors, sorted by
	// key; nil when nothing was downgraded.
	Warnings []error
}

// Materialize loads the requested environment, reveals and resolves every winning
// value, and substitutes every {{ }} reference over the composed effective
// environment. By default it returns a complete environment only when every value
// succeeds, aggregating all per-key resolution failures deterministically and
// treating a missing reference or a cycle as fatal, so it never exposes a partial
// environment and is the fail-closed path for child-process execution. Under
// IgnoreErrors it instead downgrades every per-key failure — a dangling secret, a
// missing reference, or a reference cycle — to a returned warning and omits the
// failing key, so the child still starts and inherits an omitted key from the
// ambient environment. Structural failures (a malformed manifest, unreadable
// YAML, or a flatten collision) remain fatal in both modes because they leave no
// salvageable environment.
func (m *Manager) Materialize(params MaterializeParams) (*MaterializeResult, error) {
	state, resolver, environment, err := m.prepareMaterialize(params.Environment)
	if err != nil {
		return nil, err
	}

	if params.IgnoreErrors {
		values, warnings := m.resolveEffectiveTolerant(state, resolver, environment)
		return &MaterializeResult{
			Environment: &Environment{values: values, origins: state.origins},
			Warnings:    warnings,
		}, nil
	}

	// Resolve every winning value and then substitute every {{ }} reference over
	// the composed effective environment; a materialized child always reveals.
	values, err := m.resolveEffective(state, resolver, environment)
	if err != nil {
		return nil, err
	}
	return &MaterializeResult{
		Environment: &Environment{values: values, origins: state.origins},
	}, nil
}

// prepareMaterialize runs the shared front half of materialization: it normalizes
// the environment, merges the namespaces and composes the effective environment,
// and opens a revealing resolver. Structural failures are fatal here, before
// either the strict or the lenient path resolves any value.
func (m *Manager) prepareMaterialize(
	environment string,
) (state *mergeState, resolver ValueResolver, env string, err error) {
	env, err = m.normalizeEnvironment(environment)
	if err != nil {
		return nil, nil, "", err
	}

	state, err = m.merge(env)
	if err != nil {
		return nil, nil, "", err
	}

	// Compose the complete effective environment: overlay OS overrides and union
	// OS-only keys so the child receives every variable it would see under a shell.
	m.applyOSEnvironment(state, true)

	resolver, err = m.openResolver(true)
	if err != nil {
		return nil, nil, "", err
	}
	return state, resolver, env, nil
}

// resolveLeaf dereferences each scalar item in one winning leaf value. List
// boundaries are preserved so rendering can validate and join the resolved items
// afterward. A nil resolver is an identity operation.
func resolveLeaf(
	value leafValue, resolver ValueResolver, environment string,
) (leafValue, error) {
	if value.opaque || resolver == nil {
		return value, nil
	}

	resolved := leafValue{
		items: make([]string, len(value.items)),
		list:  value.list,
	}
	for i, item := range value.items {
		result, err := resolver.Resolve(item, environment)
		if err != nil {
			return leafValue{}, err
		}
		resolved.items[i] = result
	}
	return resolved, nil
}

// materialize dereferences and renders every winning value, recording a per-key
// resolution or render failure instead of aborting so a single-key consumer can
// ignore failures in unrelated keys while whole-environment consumers aggregate
// them. Shadowed values are absent from mergeState.values and never reach the
// resolver. Errors identify the winning env-var key without exposing its value.
func materialize(
	state *mergeState, settings Settings, resolver ValueResolver, environment string,
) *materializedState {
	result := &materializedState{
		values:  make(map[string]string, len(state.values)),
		origins: state.origins,
		errs:    make(map[string]error),
	}
	for key, value := range state.values {
		resolved, err := resolveLeaf(value, resolver, environment)
		if err != nil {
			result.errs[key] = err
			continue
		}
		path := state.origins[key].Winner.Key
		rendered, err := renderLeafValue(resolved, path, settings.Delimiter)
		if err != nil {
			result.errs[key] = err
			continue
		}
		result.values[key] = rendered
	}
	return result
}

// materializationError returns every accumulated failure, one per key in sorted
// key order and each wrapped with its failing key, or nil when every key
// resolved. Reporting every unresolved key at once lets the user fix them
// together instead of one at a time.
func materializationError(errs map[string]error) error {
	if len(errs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(errs))
	for key := range errs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	failures := make([]error, 0, len(keys))
	for _, key := range keys {
		failures = append(failures, fmt.Errorf("resolving %s: %w", key, errs[key]))
	}
	return errors.Join(failures...)
}
