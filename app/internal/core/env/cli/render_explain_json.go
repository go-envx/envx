package cli

import (
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/lib/filex"
)

// explainResultJSON is the machine-readable envelope for JSON output.
type explainResultJSON struct {
	Summary explainSummaryJSON `json:"summary"`
	Entries []explainEntryJSON `json:"entries"`
}

// explainSummaryJSON is the tagged view of the aggregated diagnostic outcome.
type explainSummaryJSON struct {
	Severity string `json:"severity"`
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
}

// explainEntryJSON is the exported, tagged view of an entry used for JSON output.
type explainEntryJSON struct {
	Key       string            `json:"key"`
	Type      string            `json:"type"`
	Value     string            `json:"value"`
	Source    string            `json:"source"`
	SourceKey string            `json:"sourceKey"`
	Shadowed  []string          `json:"shadowed,omitempty"`
	Status    explainStatusJSON `json:"status"`
	Resolved  *string           `json:"resolved,omitempty"`
}

// explainStatusJSON is the tagged view of a resolution outcome for JSON output.
type explainStatusJSON struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message,omitempty"`
}

// json writes the summary and entries as JSON.
func (r explainRenderer) json(result *env.ExplainResult) error {
	entries := make([]explainEntryJSON, 0, len(result.Entries))
	for i := range result.Entries {
		entries = append(entries, r.toEntryJSON(&result.Entries[i], result.WorkspaceDir))
	}

	return r.console.WriteJSON(explainResultJSON{
		Summary: explainSummaryJSON{
			Severity: result.Summary.Severity().DisplayString(),
			Errors:   result.Summary.Errors,
			Warnings: result.Summary.Warnings,
		},
		Entries: entries,
	})
}

// toEntryJSON converts an explained entry to its tagged JSON view.
func (r explainRenderer) toEntryJSON(
	e *env.ExplainEntry,
	workspaceDir string,
) explainEntryJSON {
	source := e.Origin.Winner.Path
	if !r.absolute {
		source = filex.RelativeTo(workspaceDir, source)
	}
	shadowed := make([]string, 0, len(e.Origin.Shadowed))
	for _, s := range e.Origin.Shadowed {
		path := s.Path
		if !r.absolute {
			path = filex.RelativeTo(workspaceDir, path)
		}
		shadowed = append(shadowed, path)
	}

	var resolved *string
	if val, ok := r.resolved(e); ok {
		resolved = &val
	}

	status := r.status(e)
	return explainEntryJSON{
		Key:       e.Key,
		Type:      e.Evaluation.Kind.String(),
		Value:     e.Raw,
		Source:    source,
		SourceKey: e.Origin.Winner.Key,
		Shadowed:  shadowed,
		Status: explainStatusJSON{
			Severity: status.Level.DisplayString(),
			Code:     string(status.Code),
			Message:  status.Message,
		},
		Resolved: resolved,
	}
}
