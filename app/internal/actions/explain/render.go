package explain

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/internal/style"
	"github.com/go-envx/envx/app/pkg/str"
)

// jsonStatus is the tagged view of a resolution outcome for JSON output.
type jsonStatus struct {
	// Severity ranks the outcome (ok, warning, or error).
	Severity string `json:"severity"`
	// Code is the stable, machine-classifiable status identifier.
	Code string `json:"code"`
	// Message is a human-readable status description.
	Message string `json:"message,omitempty"`
}

// jsonEntry is the exported, tagged view of an entry used for JSON output
// (entry's own fields are unexported).
type jsonEntry struct {
	// Key is the resolved env-var key.
	Key string `json:"key"`
	// Type classifies the value.
	Type string `json:"type"`
	// Value is the literal, pre-resolution value.
	Value string `json:"value"`
	// Source is the file that provided the value.
	Source string `json:"source"`
	// SourceKey is the original key in the source file.
	SourceKey string `json:"sourceKey"`
	// Shadowed lists the files overridden by the resolved value.
	Shadowed []string `json:"shadowed,omitempty"`
	// Status is the dry-run resolution outcome.
	Status jsonStatus `json:"status"`
	// Resolved carries the materialized plaintext under --reveal. A nil pointer
	// means unresolved, distinct from a pointer to an empty resolved value.
	Resolved *string `json:"resolved,omitempty"`
}

// jsonSummary is the tagged view of the aggregated diagnostic outcome.
type jsonSummary struct {
	// Severity is the worst severity observed across entries.
	Severity string `json:"severity"`
	// Errors counts entries at error severity.
	Errors int `json:"errors"`
	// Warnings counts entries at warning severity.
	Warnings int `json:"warnings"`
}

// jsonResult is the machine-readable envelope pairing the summary with entries.
type jsonResult struct {
	// Summary aggregates the diagnostic outcome across all entries.
	Summary jsonSummary `json:"summary"`
	// Entries is the per-key explanation rows.
	Entries []jsonEntry `json:"entries"`
}

// renderParams bundles everything render needs: the printer, the structured
// result, the chosen output format, and whether resolved plaintext is shown.
type renderParams struct {
	// Printer is the styled output layer for the table, banner, and JSON.
	Printer *printer.Printer
	// Result is the structured data to render.
	Result actionResult
	// Format selects the output format ("json" or the default table).
	Format string
	// Reveal adds a RESOLVED column carrying materialized plaintext.
	Reveal bool
}

// render writes the result in the requested format ("json" or the default
// aligned table). An unrecognized format is rejected so a typo like
// --output=jsonn fails loudly.
func render(p *renderParams) error {
	switch p.Format {
	case "", "table":
		return renderTable(p.Printer, p.Result, p.Reveal)
	case "json":
		return renderJSON(p.Printer, p.Result)
	default:
		return fmt.Errorf("invalid output format %q (want table or json)", p.Format)
	}
}

// renderJSON writes the summary and entries as a symbol-free, classifiable object.
func renderJSON(p *printer.Printer, res actionResult) error {
	entries := make([]jsonEntry, 0, len(res.Entries))
	for i := range res.Entries {
		e := &res.Entries[i]
		entries = append(entries, jsonEntry{
			Key:       e.Key,
			Type:      string(e.Resolution.Kind),
			Value:     e.Literal,
			Source:    e.Source,
			SourceKey: e.SourceKey,
			Shadowed:  e.Shadowed,
			Status: jsonStatus{
				Severity: string(e.Resolution.Severity),
				Code:     e.Resolution.Code,
				Message:  e.Resolution.Message,
			},
			Resolved: resolvedPointer(e),
		})
	}

	out := jsonResult{
		Summary: jsonSummary{
			Severity: string(res.Summary.Severity()),
			Errors:   res.Summary.Errors,
			Warnings: res.Summary.Warnings,
		},
		Entries: entries,
	}
	return p.WriteJSON(out)
}

// resolvedPointer returns a pointer to the entry's resolved value when it was
// materialized, or nil otherwise, so JSON distinguishes a resolved empty value
// from an unresolved one.
func resolvedPointer(e *actionResultEntry) *string {
	if !e.Resolution.HasResolved {
		return nil
	}
	value := e.Resolution.Resolved
	return &value
}

// renderTable writes an aligned table led by a severity banner when resolution
// is incomplete. The RESOLVED column is added only when reveal is requested.
func renderTable(p *printer.Printer, res actionResult, reveal bool) error {
	if err := renderBanner(p, res.Summary); err != nil {
		return err
	}

	headers := []string{"KEY", "TYPE", "VALUE", "SOURCE", "STATUS"}
	if reveal {
		headers = append(headers, "RESOLVED")
	}

	rows := make([][]printer.Cell, 0, len(res.Entries))
	for i := range res.Entries {
		e := &res.Entries[i]
		row := []printer.Cell{
			{Text: e.Key},
			{Text: string(e.Resolution.Kind)},
			{Text: e.Literal},
			{Text: e.Source},
			{
				Text:     e.Resolution.Code,
				Severity: toStyleSeverity(e.Resolution.Severity),
			},
		}
		if reveal {
			row = append(row, printer.Cell{Text: e.Resolution.Resolved})
		}
		rows = append(rows, row)
	}

	return p.WriteTable(printer.Table{Headers: headers, Rows: rows})
}

// renderBanner writes leading severity lines to standard error when resolution
// is incomplete: an error line and a warning line, each shown independently so
// stdout carries only the table. The printer owns the ERROR/WARNING label, so
// only the message body is supplied here. A trailing blank line separates the
// banner from the table that follows.
func renderBanner(p *printer.Printer, s envmerge.ExplanationSummary) error {
	if s.Errors == 0 && s.Warnings == 0 {
		return nil
	}
	if s.Errors > 0 {
		if err := p.LogError(
			str.Pluralize(s.Errors, "value", "values") + " failed to resolve",
		); err != nil {
			return err
		}
	}
	if s.Warnings > 0 {
		if err := p.LogWarning(
			str.Pluralize(s.Warnings, "value", "values") + " resolved with warnings",
		); err != nil {
			return err
		}
	}
	return p.LogBlank()
}

// toStyleSeverity maps an envmerge severity onto a style severity, keeping the
// style package a dependency-free leaf that never imports envmerge.
func toStyleSeverity(s envmerge.Severity) style.Severity {
	switch s {
	case envmerge.SeverityOK:
		return style.SeverityOK
	case envmerge.SeverityWarning:
		return style.SeverityWarning
	case envmerge.SeverityError:
		return style.SeverityError
	default:
		return style.SeverityNone
	}
}
