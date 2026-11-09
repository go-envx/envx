package env

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// ExplainParams selects keys to diagnose and the reveal policy.
type ExplainParams struct {
	// Query selects the project and setting overrides.
	Query Query
	// Key selects one case-insensitive key; empty selects every key.
	Key string
	// Reveal retains materialized plaintext in evaluations.
	Reveal bool
}

// ExplainResult is a sorted diagnostic view of the selected winners.
type ExplainResult struct {
	// WorkspaceDir is the workspace root that source paths are relative to.
	WorkspaceDir string
	// Entries holds one entry per selected key, sorted by key.
	Entries []Entry
}

// Explain diagnoses the selected winners without aborting on a per-key
// failure. Namespace, YAML, and flatten failures remain fatal. Host-only keys
// are not enumerated.
func (s *Service) Explain(params ExplainParams) (ExplainResult, error) {
	op, err := s.begin(params.Query)
	if err != nil {
		return ExplainResult{}, err
	}
	merged, err := s.load(op, false)
	if err != nil {
		return ExplainResult{}, err
	}

	keys := slices.Sorted(maps.Keys(merged.values))
	if params.Key != "" {
		key := strings.ToUpper(params.Key)
		if _, ok := merged.values[key]; !ok {
			return ExplainResult{}, fmt.Errorf("%w: %q", ErrKeyNotFound, key)
		}
		keys = []string{key}
	}

	resolver, err := s.openResolver(params.Reveal, op.host)
	if err != nil {
		return ExplainResult{}, err
	}
	return ExplainResult{
		WorkspaceDir: s.root,
		Entries:      newEvaluator(merged, resolver, op, params.Reveal).entries(keys),
	}, nil
}
