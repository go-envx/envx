package env

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/lib/severity"
)

// MaterializeParams selects the environment to materialize and the failure
// policy.
type MaterializeParams struct {
	// Query selects the project and setting overrides.
	Query Query
	// IgnoreErrors downgrades per-key failures to warnings.
	IgnoreErrors bool
}

// MaterializeResult is a complete environment and any downgraded failures.
type MaterializeResult struct {
	// Environment is the complete materialized environment, including host-only
	// variables.
	Environment Environment
	// Warnings lists downgraded failures sorted by key; nil in fail-closed mode.
	Warnings []diagnostic.Finding
}

// failure is one key's materialization failure.
type failure struct {
	// err is the underlying failure, which never contains a value.
	err error
	// code classifies the failure; empty when no diagnostic code applies.
	code diagnostic.Code
}

// Materialize decrypts and substitutes every winner over the complete
// effective environment. By default any per-key failure is fatal, so no
// partial environment escapes. With IgnoreErrors each failure becomes a
// warning and the key keeps its host value or is omitted. Structural failures
// are fatal in both modes.
func (s *Service) Materialize(params MaterializeParams) (MaterializeResult, error) {
	op, err := s.begin(params.Query)
	if err != nil {
		return MaterializeResult{}, err
	}
	merged, err := s.load(op, true)
	if err != nil {
		return MaterializeResult{}, err
	}
	resolver, err := s.openResolver(true, op.host)
	if err != nil {
		return MaterializeResult{}, err
	}

	if params.IgnoreErrors {
		environment, warnings := s.materializeTolerant(merged, resolver, op)
		return MaterializeResult{Environment: environment, Warnings: warnings}, nil
	}
	environment, err := s.materializeStrict(merged, resolver, op)
	if err != nil {
		return MaterializeResult{}, err
	}
	return MaterializeResult{Environment: environment}, nil
}

// materializeStrict materializes and substitutes every winner, failing with every
// materialization failure, or the first substitution failure in key order.
func (s *Service) materializeStrict(
	merged *mergedEnv, resolver ValueResolver, op operation,
) (Environment, error) {
	values, failures := s.materializeLeaves(merged, resolver, op)
	if len(failures) > 0 {
		errs := make([]error, 0, len(failures))
		for _, key := range slices.Sorted(maps.Keys(failures)) {
			errs = append(errs, fmt.Errorf("resolving %s: %w", key, failures[key].err))
		}
		return Environment{}, errors.Join(errs...)
	}
	engine := newSubstituter(op, s.mapSymbols(values, merged))
	out := make(map[string]string, len(values))
	for _, key := range slices.Sorted(maps.Keys(values)) {
		composed, err := engine.resolve(key)
		if err != nil {
			return Environment{}, err
		}
		out[key] = composed
	}
	return Environment{values: out, origins: merged.origins}, nil
}

// materializeTolerant materializes and substitutes every winner, downgrading
// each failure to a warning. A failed key keeps its host value, with the host as
// its winning source, when one exists and is otherwise omitted.
func (s *Service) materializeTolerant(
	merged *mergedEnv, resolver ValueResolver, op operation,
) (Environment, []diagnostic.Finding) {
	values, failures := s.materializeLeaves(merged, resolver, op)
	engine := newSubstituter(op, s.mapSymbols(values, merged))
	out := make(map[string]string, len(values))
	origins := make(map[string]Origin, len(values))
	for key := range values {
		composed, err := engine.resolve(key)
		if err != nil {
			failures[key] = failure{err: err, code: substitutionCode(err)}
			continue
		}
		out[key] = composed
		origins[key] = merged.origins[key]
	}

	var warnings []diagnostic.Finding
	for _, key := range slices.Sorted(maps.Keys(failures)) {
		f := failures[key]
		warning := diagnostic.Finding{
			Code:        f.code,
			Level:       severity.Warn,
			Project:     op.project,
			Environment: op.environment,
			Key:         key,
			Message:     fmt.Sprintf("omitting %s: %v", key, f.err),
		}
		if hostValue, ok := op.host.Get(key); ok {
			out[key] = hostValue
			origins[key] = s.hostFallback(merged.origins[key], key)
			warning.Message = fmt.Sprintf(
				"%s: %v; keeping the value set in the environment", key, f.err,
			)
		}
		warnings = append(warnings, warning)
	}
	return Environment{values: out, origins: origins}, warnings
}

// hostFallback promotes the shadowed host source of key to winner, shadowing
// the namespace winner in its place.
func (s *Service) hostFallback(prev Origin, key string) Origin {
	if prev.Winner.Host {
		return prev
	}
	shadowed := make([]Source, 0, len(prev.Shadowed)+1)
	for _, source := range prev.Shadowed {
		if !source.Host {
			shadowed = append(shadowed, source)
		}
	}
	shadowed = append(shadowed, prev.Winner)
	return Origin{Winner: Source{Key: key, Host: true}, Shadowed: shadowed}
}

// materializeLeaves resolves and renders every winner, recording per-key
// failures instead of aborting. Shadowed values never reach the resolver.
func (s *Service) materializeLeaves(
	merged *mergedEnv, resolver ValueResolver, op operation,
) (values map[string]string, failures map[string]failure) {
	values = make(map[string]string, len(merged.values))
	failures = make(map[string]failure)
	for key, leaf := range merged.values {
		resolved, err := resolveLeaf(leaf, resolver, op.environment)
		if err != nil {
			failures[key] = failure{err: err, code: diagnostic.SecretReferenceIsUnresolved}
			continue
		}
		path := merged.origins[key].Winner.Key
		rendered, err := renderLeaf(resolved, path, op.settings.delimiter)
		if err != nil {
			failures[key] = failure{err: err}
			continue
		}
		values[key] = rendered
	}
	return values, failures
}

// mapSymbols is a symbol table over already materialized values.
func (s *Service) mapSymbols(values map[string]string, merged *mergedEnv) symbolTable {
	return symbolTable{
		declared: func(name string) bool { _, ok := values[name]; return ok },
		opaque:   merged.opaque,
		value:    func(name string) (string, error) { return values[name], nil },
	}
}
