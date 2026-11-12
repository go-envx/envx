package env

import (
	"maps"
	"slices"

	"github.com/go-envx/envx/app/internal/core/workspace"
)

// DiffParams selects two environments of one project to compare.
type DiffParams struct {
	// Project names the declared project; empty selects the only project.
	Project string
	// EnvironmentA is the "before" environment; empty uses the default.
	EnvironmentA string
	// EnvironmentB is the "after" environment; empty uses the default.
	EnvironmentB string
	// Options are setting overrides that win over the manifest; their
	// environment is the default for an empty side.
	Options workspace.Settings
}

// Change is one differing key and the declared literal of each side.
type Change struct {
	// Key is the variable name.
	Key string
	// Before is the value in EnvironmentA; empty for an addition.
	Before string
	// After is the value in EnvironmentB; empty for a removal.
	After string
}

// DiffResult is the comparison of two environments, each list sorted by key.
type DiffResult struct {
	// Added lists keys present only in EnvironmentB.
	Added []Change
	// Removed lists keys present only in EnvironmentA.
	Removed []Change
	// Changed lists keys present in both with differing values.
	Changed []Change
}

// Diff compares the declared literals of two environments over one read of the
// base namespaces. It never resolves secrets, so a changed reference is visible
// even when both decrypt to the same plaintext. Host-only keys are not compared.
func (s *Service) Diff(params DiffParams) (DiffResult, error) {
	op, err := s.begin(params.Project, params.Options)
	if err != nil {
		return DiffResult{}, err
	}
	from, err := s.diffEnvironment(op, params.EnvironmentA)
	if err != nil {
		return DiffResult{}, err
	}
	to, err := s.diffEnvironment(op, params.EnvironmentB)
	if err != nil {
		return DiffResult{}, err
	}

	namespaces, err := s.loadNamespaces(op.includes)
	if err != nil {
		return DiffResult{}, err
	}
	before, err := s.diffSide(namespaces, from)
	if err != nil {
		return DiffResult{}, err
	}
	after, err := s.diffSide(namespaces, to)
	if err != nil {
		return DiffResult{}, err
	}

	keys := make(map[string]struct{}, len(before)+len(after))
	for key := range before {
		keys[key] = struct{}{}
	}
	for key := range after {
		keys[key] = struct{}{}
	}
	var result DiffResult
	for _, key := range slices.Sorted(maps.Keys(keys)) {
		a, inBefore := before[key]
		b, inAfter := after[key]
		switch {
		case inBefore && inAfter && a == b:
			continue
		case inBefore && inAfter:
			result.Changed = append(result.Changed, Change{Key: key, Before: a, After: b})
		case inBefore:
			result.Removed = append(result.Removed, Change{Key: key, Before: a})
		default:
			result.Added = append(result.Added, Change{Key: key, After: b})
		}
	}
	return result, nil
}

// diffEnvironment validates an explicit side or falls back to op's default.
func (s *Service) diffEnvironment(op operation, environment string) (operation, error) {
	if environment == "" {
		return op, nil
	}
	selected, err := s.environment(&environment)
	if err != nil {
		return operation{}, err
	}
	op.environment = selected
	return op, nil
}

// diffSide merges one side and renders each key's declared literal, which
// still validates list rendering.
func (s *Service) diffSide(
	namespaces []namespace, op operation,
) (map[string]string, error) {
	merged, err := s.merge(namespaces, op)
	if err != nil {
		return nil, err
	}
	merged.applyHost(op, false)

	out := make(map[string]string, len(merged.values))
	for _, key := range slices.Sorted(maps.Keys(merged.values)) {
		path := merged.origins[key].Winner.Key
		literal, err := renderLeaf(merged.values[key], path, op.settings.delimiter)
		if err != nil {
			return nil, err
		}
		out[key] = literal
	}
	return out, nil
}
