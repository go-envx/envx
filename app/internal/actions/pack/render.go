package pack

import (
	"fmt"
	"path/filepath"
	"strings"

	engine "github.com/go-envx/envx/app/internal/pack"
	"github.com/go-envx/envx/app/internal/utils/printer"
)

// render writes a human summary of a completed pack through the shared printer:
// the destination, the included environments and projects, and the copied files.
func render(pr *printer.Printer, result engine.Result) error {
	return pr.LogMessage(summary(result))
}

// summary formats the pack result: a header naming the destination and the
// included environments, then the copied files, then the container run hint. It
// is a pure function of the result so it stays trivially testable.
func summary(result engine.Result) string {
	lines := []string{
		fmt.Sprintf(
			"Packed %d files into %s for environments [%s]:",
			len(result.Files),
			result.OutDir,
			strings.Join(result.Environments, ", "),
		),
	}
	for _, f := range result.Files {
		lines = append(lines, "  "+f)
	}
	lines = append(lines,
		"",
		"Run one of the packed projects:",
		fmt.Sprintf(
			"  envx run %s --env %s --config %s -- <command>",
			firstOr(result.Projects, "<project>"),
			firstOr(result.Environments, "<env>"),
			filepath.Join(result.OutDir, result.ManifestFile),
		),
	)
	return strings.Join(lines, "\n")
}

// firstOr returns the first element of values, or fallback when values is empty,
// so the run hint names a concrete project and environment when one exists and a
// placeholder otherwise.
func firstOr(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return values[0]
}
