package cli

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/features/validate"
	"github.com/go-envx/envx/app/internal/utils/printer"
)

// table writes an aligned table of findings led by a severity banner. A clean
// report prints a single confirmation line instead of an empty table.
func (r validateRenderer) table(report validate.Report) error {
	if len(report.Findings) == 0 {
		return r.console.LogMessage("no validation problems found")
	}

	if err := r.banner(report); err != nil {
		return err
	}

	headers := []string{"SCOPE", "KEY", "STATUS", "MESSAGE"}
	rows := make([][]printer.Cell, 0, len(report.Findings))
	for _, f := range report.Findings {
		rows = append(rows, []printer.Cell{
			{Text: r.scope(f)},
			{Text: f.Key},
			{Text: f.Code, Severity: f.Severity},
			{Text: f.Message},
		})
	}
	table := printer.Table{Headers: headers, Rows: rows}
	if err := r.console.WriteTable(table); err != nil {
		return err
	}

	// When the report fails, the process boundary prints a terse "validation
	// failed" verdict on standard error after this renders. A trailing blank on
	// standard error sets that verdict apart from the table without adding a blank
	// line to the table data on standard output.
	if report.Failed {
		return r.console.LogBlank()
	}
	return nil
}

// scope renders a finding's scope: the "project/environment" it was observed in
// for a reference finding, or "secrets" for a store-level finding. "store" is an
// internal term; users know the secrets store as secrets.yaml, so the scope reads
// "secrets" rather than exposing the implementation concept.
func (r validateRenderer) scope(f validate.Finding) string {
	if f.Project == "" && f.Environment == "" {
		return "secrets"
	}
	return f.Project + "/" + f.Environment
}

// banner writes leading severity lines to standard error summarizing the counts,
// so stdout carries only the table. A trailing blank line separates the banner
// from the table that follows.
func (r validateRenderer) banner(report validate.Report) error {
	if report.Errors == 0 && report.Warnings == 0 {
		return nil
	}
	if report.Errors > 0 {
		if err := r.console.LogError(
			fmt.Sprintf("%d error(s) found", report.Errors),
		); err != nil {
			return err
		}
	}
	if report.Warnings > 0 {
		if err := r.console.LogWarning(
			fmt.Sprintf("%d warning(s) found", report.Warnings),
		); err != nil {
			return err
		}
	}
	return r.console.LogBlank()
}
