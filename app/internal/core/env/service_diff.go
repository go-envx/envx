package env

import (
	"errors"
	"fmt"
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
	// Reveal compares resolved values instead of declared literals.
	Reveal bool
	// Options are setting overrides that win over the manifest; their
	// environment is the default for an empty side.
	Options workspace.Settings
}

// Change is one differing key and the text each side compared by: the
// declared literal, or the resolved value when revealing.
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

// Diff compares two environments over one read of the base namespaces. Masked,
// it compares declared literals, so a changed reference is visible even when
// both decrypt to the same plaintext. Revealed, it compares resolved values and
// any value that does not resolve on either side is fatal. Host-only keys are
// not compared.
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
	resolver, err := s.newValueResolver(params.Reveal, op.host)
	if err != nil {
		return DiffResult{}, err
	}

	before, err := s.diffSide(namespaces, resolver, from, params.Reveal)
	if err != nil {
		return DiffResult{}, err
	}
	after, err := s.diffSide(namespaces, resolver, to, params.Reveal)
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

// diffSide merges and evaluates one side, returning each key's compared text.
// Masked, list rendering is still validated; revealed, every value must
// resolve.
func (s *Service) diffSide(
	namespaces []namespace, resolver ValueResolver, op operation, reveal bool,
) (map[string]string, error) {
	merged, err := s.merge(namespaces, op)
	if err != nil {
		return nil, err
	}
	merged.applyHost(op, false)

	keys := slices.Sorted(maps.Keys(merged.values))
	entries := newEvaluator(merged, resolver, op, reveal).entries(keys)
	out := make(map[string]string, len(entries))
	var unresolved []error
	for i := range entries {
		entry := &entries[i]
		compared := entry.Evaluation.Value
		if !reveal {
			leaf := merged.values[entry.Key]
			path := merged.origins[entry.Key].Winner.Key
			if compared, err = renderLeaf(leaf, path, op.settings.delimiter); err != nil {
				return nil, err
			}
		} else if !entry.Evaluation.Resolved {
			unresolved = append(unresolved, s.unresolvedError(entry))
		}
		out[entry.Key] = compared
	}
	if len(unresolved) > 0 {
		return nil, fmt.Errorf(
			"environment %q: %w", op.environment, errors.Join(unresolved...),
		)
	}
	return out, nil
}

// unresolvedError describes an unresolved entry without its value.
func (s *Service) unresolvedError(entry *ExplainEntry) error {
	message := "did not resolve"
	if !entry.Evaluation.Diagnostic.IsZero() {
		message = entry.Evaluation.Diagnostic.Message
	}
	return fmt.Errorf("resolving %s: %w: %s", entry.Key, ErrUnresolved, message)
}
