package explain

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/go-envx/envx/app/internal/envmerge"
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

// renderParams bundles everything render needs: the output sink, the structured
// result, the chosen output format, and whether resolved plaintext is shown.
type renderParams struct {
	// Writer is the output sink to render to.
	Writer io.Writer
	// Result is the structured data to render.
	Result actionResult
	// Format selects the output format ("json" or the default table).
	Format string
	// Reveal adds a RESOLVED column carrying materialized plaintext.
	Reveal bool
}

// render writes the result to p.Writer in the requested format ("json" or the
// default aligned table). An unrecognized format is rejected so a typo like
// --output=jsonn fails loudly.
func render(p *renderParams) error {
	switch p.Format {
	case "", "table":
		return renderTable(p.Writer, p.Result, p.Reveal)
	case "json":
		return renderJSON(p.Writer, p.Result)
	default:
		return fmt.Errorf("invalid output format %q (want table or json)", p.Format)
	}
}

// renderJSON writes the summary and entries as a symbol-free, classifiable object.
func renderJSON(w io.Writer, res actionResult) error {
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
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
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
func renderTable(w io.Writer, res actionResult, reveal bool) error {
	if banner := bannerLine(res.Summary); banner != "" {
		if _, err := fmt.Fprintln(w, banner); err != nil {
			return err
		}
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	header := "KEY\tTYPE\tVALUE\tSOURCE\tSTATUS"
	if reveal {
		header += "\tRESOLVED"
	}
	if _, err := fmt.Fprintln(tw, header); err != nil {
		return err
	}

	for i := range res.Entries {
		e := &res.Entries[i]
		row := fmt.Sprintf(
			"%s\t%s\t%s\t%s\t%s",
			e.Key, string(e.Resolution.Kind), e.Literal, e.Source, e.Resolution.Code,
		)
		if reveal {
			row += "\t" + e.Resolution.Resolved
		}
		if _, err := fmt.Fprintln(tw, row); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// bannerLine returns a leading summary line when resolution is incomplete, or an
// empty string when every value resolved. It is symbol-free.
func bannerLine(s envmerge.ExplanationSummary) string {
	switch {
	case s.Errors > 0:
		return fmt.Sprintf(
			"ERROR: %d value(s) failed to resolve, %d warning(s)", s.Errors, s.Warnings,
		)
	case s.Warnings > 0:
		return fmt.Sprintf("WARNING: %d value(s) resolved with warnings", s.Warnings)
	default:
		return ""
	}
}
