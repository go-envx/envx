package cli

import (
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/lib/printer"
	"github.com/go-envx/envx/app/internal/lib/str"
)

// table writes an aligned table led by a banner when resolution is incomplete.
func (r explainRenderer) table(result *env.ExplainResult) error {
	if err := r.toTableBanner(result.Summary); err != nil {
		return err
	}

	headers := []string{"KEY", "TYPE", "VALUE", "SOURCE", "STATUS"}
	if r.reveal {
		headers = append(headers, "RESOLVED")
	}

	rows := make([][]printer.Cell, 0, len(result.Entries))
	for i := range result.Entries {
		rows = append(rows, r.toTableRow(&result.Entries[i], result.WorkspaceDir))
	}

	return r.console.WriteTable(printer.Table{Headers: headers, Rows: rows})
}

// toTableRow formats one row of explain output.
func (r explainRenderer) toTableRow(
	e *env.ExplainEntry,
	workspaceDir string,
) []printer.Cell {
	source := e.Origin.Winner.Path
	if !r.absolute {
		source = filex.RelativeTo(workspaceDir, source)
	}
	status := r.status(e)
	row := []printer.Cell{
		{Text: e.Key},
		{Text: e.Evaluation.Kind.String()},
		{Text: e.Raw},
		{Text: source},
		{
			Text:     string(status.Code),
			Severity: status.Level,
		},
	}
	if r.reveal {
		resolved, _ := r.resolved(e)
		row = append(row, printer.Cell{Text: resolved})
	}
	return row
}

// toTableBanner writes leading severity lines to standard error when resolution is
// incomplete.
func (r explainRenderer) toTableBanner(s env.ExplanationSummary) error {
	if s.Errors == 0 && s.Warnings == 0 {
		return nil
	}
	if s.Errors > 0 {
		if err := r.console.LogError(
			str.Pluralize(s.Errors, "value", "values") + " failed to resolve",
		); err != nil {
			return err
		}
	}
	if s.Warnings > 0 {
		if err := r.console.LogWarning(
			str.Pluralize(s.Warnings, "value", "values") + " resolved with warnings",
		); err != nil {
			return err
		}
	}
	return r.console.LogBlank()
}
