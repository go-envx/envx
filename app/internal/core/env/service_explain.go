package env

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/severity"
)

// ExplainParams selects keys to diagnose and the reveal policy.
type ExplainParams struct {
	// Project names the declared project; empty selects the only project.
	Project string
	// Key selects one case-insensitive key; empty selects every key.
	Key string
	// Reveal retains materialized plaintext in evaluations.
	Reveal bool
	// Options are setting overrides that win over the manifest.
	Options workspace.Settings
}

// ExplainResult is a sorted diagnostic view of the selected winners.
type ExplainResult struct {
	// WorkspaceDir is the workspace root that source paths are relative to.
	WorkspaceDir string
	// Entries holds one entry per selected key, sorted by key.
	Entries []ExplainEntry
	// Summary counts the entries by severity.
	Summary ExplanationSummary
}

// ExplanationSummary counts explained entries that failed or warned.
type ExplanationSummary struct {
	// Errors is the number of entries at error severity.
	Errors int
	// Warnings is the number of entries at warning severity.
	Warnings int
}

// Severity returns the worst severity the summary counts.
func (s ExplanationSummary) Severity() severity.Level {
	switch {
	case s.Errors > 0:
		return severity.Error
	case s.Warnings > 0:
		return severity.Warn
	default:
		return severity.OK
	}
}

// Explain diagnoses the selected winners without aborting on a per-key
// failure. Namespace, YAML, and flatten failures remain fatal. Host-only keys
// are not enumerated.
func (s *Service) Explain(params ExplainParams) (ExplainResult, error) {
	op, err := s.begin(params.Project, params.Options)
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

	resolver, err := s.newValueResolver(params.Reveal, op.host)
	if err != nil {
		return ExplainResult{}, err
	}
	entries := newEvaluator(merged, resolver, op, params.Reveal).entries(keys)
	var summary ExplanationSummary
	for i := range entries {
		switch entries[i].Evaluation.Diagnostic.Level {
		case severity.Error:
			summary.Errors++
		case severity.Warn:
			summary.Warnings++
		default:
		}
	}
	return ExplainResult{
		WorkspaceDir: s.root,
		Entries:      entries,
		Summary:      summary,
	}, nil
}
