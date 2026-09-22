package validate

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/style"
	engine "github.com/go-envx/envx/app/internal/validate"
)

// jsonFinding is the exported, tagged view of a finding for JSON output (the
// engine's fields are exported but re-tagged here for stable JSON keys).
type jsonFinding struct {
	// Severity ranks the finding.
	Severity string `json:"severity"`
	// Project is the project a reference finding was observed in; omitted for a
	// store-level finding.
	Project string `json:"project,omitempty"`
	// Environment is the environment a reference finding was observed in; omitted
	// for a store-level finding.
	Environment string `json:"environment,omitempty"`
	// Key identifies the finding's subject.
	Key string `json:"key"`
	// Code is the stable status identifier.
	Code string `json:"code"`
	// Message is the human-readable description.
	Message string `json:"message,omitempty"`
}

// jsonSummary is the tagged aggregate outcome.
type jsonSummary struct {
	// Failed reports whether the run exits non-zero.
	Failed bool `json:"failed"`
	// Errors counts findings at error severity.
	Errors int `json:"errors"`
	// Warnings counts findings at warning severity.
	Warnings int `json:"warnings"`
}

// jsonReport is the machine-readable envelope pairing the summary with findings.
type jsonReport struct {
	// Summary aggregates the outcome across all findings.
	Summary jsonSummary `json:"summary"`
	// Findings is the per-problem list.
	Findings []jsonFinding `json:"findings"`
}

// renderParams bundles everything render needs: the printer, the graded report,
// and the chosen output format.
type renderParams struct {
	// Printer is the styled output layer for the table, banner, and JSON.
	Printer *printer.Printer
	// Report is the graded validation outcome to render.
	Report engine.Report
	// Format selects the output format ("json" or the default table).
	Format string
}

// render writes the report in the requested format ("json" or the default
// aligned table). An unrecognized format is rejected so a typo like
// --output=jsonn fails loudly.
func render(p *renderParams) error {
	switch p.Format {
	case "", "table":
		return renderTable(p.Printer, p.Report)
	case "json":
		return renderJSON(p.Printer, p.Report)
	default:
		return fmt.Errorf("invalid output format %q (want table or json)", p.Format)
	}
}

// renderJSON writes the summary and findings as a symbol-free, classifiable
// object so a CI pipeline can parse the outcome.
func renderJSON(p *printer.Printer, report engine.Report) error {
	findings := make([]jsonFinding, 0, len(report.Findings))
	for _, f := range report.Findings {
		findings = append(findings, jsonFinding{
			Severity:    string(f.Severity),
			Project:     f.Project,
			Environment: f.Environment,
			Key:         f.Key,
			Code:        f.Code,
			Message:     f.Message,
		})
	}
	return p.WriteJSON(jsonReport{
		Summary: jsonSummary{
			Failed:   report.Failed,
			Errors:   report.Errors,
			Warnings: report.Warnings,
		},
		Findings: findings,
	})
}

// renderTable writes an aligned table of findings led by a severity banner. A
// clean report prints a single confirmation line instead of an empty table.
func renderTable(p *printer.Printer, report engine.Report) error {
	if len(report.Findings) == 0 {
		return p.LogMessage("no validation problems found")
	}

	if err := renderBanner(p, report); err != nil {
		return err
	}

	headers := []string{"SCOPE", "KEY", "STATUS", "MESSAGE"}
	rows := make([][]printer.Cell, 0, len(report.Findings))
	for _, f := range report.Findings {
		rows = append(rows, []printer.Cell{
			{Text: scope(f)},
			{Text: f.Key},
			{Text: f.Code, Severity: toStyleSeverity(f.Severity)},
			{Text: f.Message},
		})
	}
	if err := p.WriteTable(printer.Table{Headers: headers, Rows: rows}); err != nil {
		return err
	}

	// When the report fails, the process boundary prints a terse "validation
	// failed" verdict on standard error after this renders. A trailing blank on
	// standard error sets that verdict apart from the table without adding a blank
	// line to the table data on standard output.
	if report.Failed {
		return p.LogBlank()
	}
	return nil
}

// scope renders a finding's scope: the "project/environment" it was observed in
// for a reference finding, or "secrets" for a store-level finding. "store" is an
// internal term; users know the secrets store as secrets.yaml, so the scope reads
// "secrets" rather than exposing the implementation concept.
func scope(f engine.Finding) string {
	if f.Project == "" && f.Environment == "" {
		return "secrets"
	}
	return f.Project + "/" + f.Environment
}

// renderBanner writes leading severity lines to standard error summarizing the
// counts, so stdout carries only the table. A trailing blank line separates the
// banner from the table that follows.
func renderBanner(p *printer.Printer, report engine.Report) error {
	if report.Errors == 0 && report.Warnings == 0 {
		return nil
	}
	if report.Errors > 0 {
		if err := p.LogError(
			fmt.Sprintf("%d error(s) found", report.Errors),
		); err != nil {
			return err
		}
	}
	if report.Warnings > 0 {
		if err := p.LogWarning(
			fmt.Sprintf("%d warning(s) found", report.Warnings),
		); err != nil {
			return err
		}
	}
	return p.LogBlank()
}

// toStyleSeverity maps a validate severity onto a style severity, keeping the
// style package a dependency-free leaf that never imports validate.
func toStyleSeverity(s engine.Severity) style.Severity {
	switch s {
	case engine.SeverityError:
		return style.SeverityError
	case engine.SeverityWarning:
		return style.SeverityWarning
	default:
		return style.SeverityNone
	}
}
