package cli

import (
	"github.com/go-envx/envx/app/internal/features/validate"
)

// validateFindingJSON is the exported, tagged view of a finding for JSON output
// (the domain fields are exported but re-tagged here for stable JSON keys).
type validateFindingJSON struct {
	Severity    string `json:"severity"`
	Project     string `json:"project,omitempty"`
	Environment string `json:"environment,omitempty"`
	Key         string `json:"key"`
	Code        string `json:"code"`
	Message     string `json:"message,omitempty"`
}

// validateSummaryJSON is the tagged aggregate outcome.
type validateSummaryJSON struct {
	Failed   bool `json:"failed"`
	Errors   int  `json:"errors"`
	Warnings int  `json:"warnings"`
}

// validateReportJSON is the machine-readable envelope pairing the summary with
// findings.
type validateReportJSON struct {
	Summary  validateSummaryJSON   `json:"summary"`
	Findings []validateFindingJSON `json:"findings"`
}

// json writes the summary and findings as a symbol-free, classifiable object so
// a CI pipeline can parse the outcome.
func (r validateRenderer) json(report validate.Report) error {
	return r.console.WriteJSON(validateReportJSON{
		Summary: validateSummaryJSON{
			Failed:   report.Failed,
			Errors:   report.Errors,
			Warnings: report.Warnings,
		},
		Findings: r.toFindingsJSON(report.Findings),
	})
}

// toFindingsJSON converts findings to their tagged JSON view.
func (r validateRenderer) toFindingsJSON(in []validate.Finding) []validateFindingJSON {
	out := make([]validateFindingJSON, 0, len(in))
	for _, f := range in {
		out = append(out, validateFindingJSON{
			Severity:    f.Severity.DisplayString(),
			Project:     f.Project,
			Environment: f.Environment,
			Key:         f.Key,
			Code:        f.Code,
			Message:     f.Message,
		})
	}
	return out
}
