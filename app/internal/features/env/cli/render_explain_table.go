package cli

import (
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/utils/filex"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
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
	e *env.ExplanationEntry,
	workspaceDir string,
) []printer.Cell {
	source := e.Origin.Winner.File
	if !r.absolute {
		source = filex.RelativeTo(workspaceDir, source)
	}
	row := []printer.Cell{
		{Text: e.Key},
		{Text: e.Resolution.Kind.String()},
		{Text: e.Literal},
		{Text: source},
		{
			Text:     e.Resolution.Code,
			Severity: e.Resolution.Severity,
		},
	}
	if r.reveal {
		row = append(row, printer.Cell{Text: e.Resolution.Resolved})
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
