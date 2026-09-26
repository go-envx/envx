package cli

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/utils/printer"
)

// summaryParams holds the inputs needed to format the scaffold summary.
type summaryParams struct {
	Template  string
	TargetDir string
	Written   []string
}

// render writes a human summary of a completed scaffold through the shared printer.
func render(pr *printer.Printer, params summaryParams) error {
	return pr.LogMessage(summary(params))
}

// summary formats the scaffold summary: the written files and the first command
// to try, so the user can start exploring immediately. It is a pure function of
// the result so it stays trivially testable.
func summary(params summaryParams) string {
	lines := []string{
		fmt.Sprintf(
			"Scaffolded %s into %s/ (%d files):",
			params.Template,
			params.TargetDir,
			len(params.Written),
		),
	}
	for _, f := range params.Written {
		lines = append(lines, "  "+f)
	}
	lines = append(lines,
		"",
		"Try it:",
		"  cd "+params.TargetDir,
		"  envx get api-service DATABASE_HOST",
	)
	return strings.Join(lines, "\n")
}
