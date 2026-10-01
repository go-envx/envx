package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/arg"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	explainUsage = "explain <project> [key]"
	explainShort = "Show where each resolved value came from"
	explainLong  = `
		Explain resolves a project's environment and reports, for each key, its
		type, literal value, the file it was resolved from, and a resolution
		status. It never aborts on a failed value: an unresolved key is reported
		through its status and the command still exits 0. With no key it explains
		every key; with a key it explains just that one.

		Secret references are classified without materializing plaintext by
		default; pass --reveal to add a RESOLVED column with their decrypted
		values. Source paths are shown relative to envx.yaml unless --absolute is
		set. Use --output=json for machine-readable output.
	`
	explainExample = `
		envx explain api-service
		envx explain api-service DATABASE_HOST
		envx explain api-service --reveal
		envx explain api-service --absolute
		envx explain api-service --output=json
	`
)

// NewExplainCommand builds the "explain" command.
func NewExplainCommand(f Factory) *cobra.Command {
	var output string
	var reveal bool
	var absolute bool

	cmd := &cobra.Command{
		Use:     explainUsage,
		Short:   explainShort,
		Long:    str.Dedent(explainLong),
		Example: str.Dedent(explainExample, 2),
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			key := arg.Optional(args, 1)

			in := core.GetInput(cmd.Flags())

			envService, err := f.EnvService(in, project)
			if err != nil {
				return err
			}

			result, err := envService.Explain(env.ExplainParams{
				Key:    key,
				Reveal: reveal,
			})
			if err != nil {
				return err
			}

			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			return outputExplain(
				console, result, envService.WorkspaceDir(), output, reveal, absolute,
			)
		},
	}

	flags.Bind(cmd.Flags(), &output, &flags.Output)
	flags.Bind(cmd.Flags(), &reveal, &env.Reveal)
	flags.Bind(cmd.Flags(), &absolute, &env.Absolute)

	env.RegisterFlags(cmd.Flags(),
		env.WithEnv,
		env.WithRequireOverlays,
		env.WithPrefix,
		env.WithSuffix,
		env.WithDelimiter,
		env.WithOverload,
		env.WithReferencePattern,
	)

	return cmd
}

// outputExplain renders the explanation in table or JSON format.
func outputExplain(
	console *printer.Printer,
	result *env.ExplainResult,
	workspaceDir, format string,
	reveal, absolute bool,
) error {
	switch format {
	case "", "table":
		return renderExplainTable(console, result, workspaceDir, reveal, absolute)
	case "json":
		return renderExplainJSON(console, result, workspaceDir, absolute)
	default:
		return fmt.Errorf("invalid output format %q (want table or json)", format)
	}
}

// jsonExplainStatus is the tagged view of a resolution outcome for JSON output.
type jsonExplainStatus struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message,omitempty"`
}

// jsonExplainEntry is the exported, tagged view of an entry used for JSON output.
type jsonExplainEntry struct {
	Key       string            `json:"key"`
	Type      string            `json:"type"`
	Value     string            `json:"value"`
	Source    string            `json:"source"`
	SourceKey string            `json:"sourceKey"`
	Shadowed  []string          `json:"shadowed,omitempty"`
	Status    jsonExplainStatus `json:"status"`
	Resolved  *string           `json:"resolved,omitempty"`
}

// jsonExplainSummary is the tagged view of the aggregated diagnostic outcome.
type jsonExplainSummary struct {
	Severity string `json:"severity"`
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
}

// jsonExplainResult is the machine-readable envelope for JSON output.
type jsonExplainResult struct {
	Summary jsonExplainSummary `json:"summary"`
	Entries []jsonExplainEntry `json:"entries"`
}

// renderExplainJSON writes the summary and entries as JSON.
func renderExplainJSON(
	p *printer.Printer,
	result *env.ExplainResult,
	workspaceDir string,
	absolute bool,
) error {
	entries := make([]jsonExplainEntry, 0, len(result.Entries))
	for i := range result.Entries {
		e := &result.Entries[i]
		source := e.Origin.Winner.File
		if !absolute {
			source = sourcePath(source, workspaceDir)
		}
		shadowed := make([]string, 0, len(e.Origin.Shadowed))
		for _, s := range e.Origin.Shadowed {
			path := s.File
			if !absolute {
				path = sourcePath(path, workspaceDir)
			}
			shadowed = append(shadowed, path)
		}

		var resolved *string
		if e.Resolution.HasResolved {
			val := e.Resolution.Resolved
			resolved = &val
		}

		entries = append(entries, jsonExplainEntry{
			Key:       e.Key,
			Type:      e.Resolution.Kind.String(),
			Value:     e.Literal,
			Source:    source,
			SourceKey: e.Origin.Winner.Key,
			Shadowed:  shadowed,
			Status: jsonExplainStatus{
				Severity: e.Resolution.Severity.DisplayString(),
				Code:     e.Resolution.Code,
				Message:  e.Resolution.Message,
			},
			Resolved: resolved,
		})
	}

	out := jsonExplainResult{
		Summary: jsonExplainSummary{
			Severity: result.Summary.Severity().DisplayString(),
			Errors:   result.Summary.Errors,
			Warnings: result.Summary.Warnings,
		},
		Entries: entries,
	}
	return p.WriteJSON(out)
}

// renderExplainTable writes an aligned table led by a banner when resolution
// is incomplete.
func renderExplainTable(
	p *printer.Printer,
	result *env.ExplainResult,
	workspaceDir string,
	reveal, absolute bool,
) error {
	if err := renderExplainBanner(p, result.Summary); err != nil {
		return err
	}

	headers := []string{"KEY", "TYPE", "VALUE", "SOURCE", "STATUS"}
	if reveal {
		headers = append(headers, "RESOLVED")
	}

	rows := make([][]printer.Cell, 0, len(result.Entries))
	for i := range result.Entries {
		e := &result.Entries[i]
		source := e.Origin.Winner.File
		if !absolute {
			source = sourcePath(source, workspaceDir)
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
		if reveal {
			row = append(row, printer.Cell{Text: e.Resolution.Resolved})
		}
		rows = append(rows, row)
	}

	return p.WriteTable(printer.Table{Headers: headers, Rows: rows})
}

// renderExplainBanner writes leading severity lines to standard error when
// resolution is incomplete.
func renderExplainBanner(p *printer.Printer, s env.ExplanationSummary) error {
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

// sourcePath renders a source path relative to the workspace root when it lies
// inside it, and as an absolute path otherwise.
func sourcePath(path, workspaceDir string) string {
	rel, err := filepath.Rel(workspaceDir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}
