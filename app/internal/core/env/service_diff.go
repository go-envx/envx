package env

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// DiffParams selects two environments of one project to compare.
type DiffParams struct {
	// Query selects the project and setting overrides; its environment is the
	// default for an empty side.
	Query Query
	// From is the "before" environment; empty uses the default.
	From string
	// To is the "after" environment; empty uses the default.
	To string
	// Reveal compares resolved values instead of declared literals.
	Reveal bool
}

// Change is one differing key. Before is nil for an addition and After is nil
// for a removal.
type Change struct {
	// Key is the variable name.
	Key string
	// Before is the entry in From.
	Before *Entry
	// After is the entry in To.
	After *Entry
}

// DiffResult is the sorted comparison of two environments.
type DiffResult struct {
	// WorkspaceDir is the workspace root that source paths are relative to.
	WorkspaceDir string
	// Changes lists added, removed, and changed keys, sorted by key.
	Changes []Change
}

// diffEntry pairs an entry with the text it is compared by.
type diffEntry struct {
	// entry is the evaluated winner.
	entry Entry
	// compared is the declared literal, or the resolved value when revealing.
	compared string
}

// Diff compares two environments over one read of the base namespaces. Masked,
// it compares declared literals, so a changed reference is visible even when
// both decrypt to the same plaintext. Revealed, it compares resolved values and
// any value that does not resolve on either side is fatal. Host-only keys are
// not compared.
func (s *Service) Diff(params DiffParams) (DiffResult, error) {
	op, err := s.begin(params.Query)
	if err != nil {
		return DiffResult{}, err
	}
	from, err := s.diffEnvironment(op, params.From)
	if err != nil {
		return DiffResult{}, err
	}
	to, err := s.diffEnvironment(op, params.To)
	if err != nil {
		return DiffResult{}, err
	}

	namespaces, err := s.loadNamespaces(op.includes)
	if err != nil {
		return DiffResult{}, err
	}
	resolver, err := s.openResolver(params.Reveal, op.host)
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
	var changes []Change
	for _, key := range slices.Sorted(maps.Keys(keys)) {
		a, inBefore := before[key]
		b, inAfter := after[key]
		switch {
		case inBefore && inAfter && a.compared == b.compared:
			continue
		case inBefore && inAfter:
			changes = append(changes, Change{Key: key, Before: &a.entry, After: &b.entry})
		case inBefore:
			changes = append(changes, Change{Key: key, Before: &a.entry})
		default:
			changes = append(changes, Change{Key: key, After: &b.entry})
		}
	}
	return DiffResult{WorkspaceDir: s.root, Changes: changes}, nil
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

// diffSide merges and evaluates one side. Masked, list rendering is still
// validated; revealed, every value must resolve.
func (s *Service) diffSide(
	namespaces []namespace, resolver ValueResolver, op operation, reveal bool,
) (map[string]diffEntry, error) {
	merged, err := s.merge(namespaces, op)
	if err != nil {
		return nil, err
	}
	merged.applyHost(op, false)

	keys := slices.Sorted(maps.Keys(merged.values))
	entries := newEvaluator(merged, resolver, op, reveal).entries(keys)
	out := make(map[string]diffEntry, len(entries))
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
		out[entry.Key] = diffEntry{entry: *entry, compared: compared}
	}
	if len(unresolved) > 0 {
		return nil, fmt.Errorf(
			"environment %q: %w", op.environment, errors.Join(unresolved...),
		)
	}
	return out, nil
}

// unresolvedError describes an unresolved entry without its value.
func (s *Service) unresolvedError(entry *Entry) error {
	message := "did not resolve"
	if !entry.Evaluation.Diagnostic.IsZero() {
		message = entry.Evaluation.Diagnostic.Message
	}
	return fmt.Errorf("resolving %s: %w: %s", entry.Key, ErrUnresolved, message)
}
