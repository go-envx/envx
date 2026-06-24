package explain

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// redacted is the placeholder shown for masked (non-revealed) values.
const redacted = "********"

// -------------------------------------------------------------------------------------

// jsonEntry is the exported, tagged view of an entry used for JSON output
// (entry's own fields are unexported).
type jsonEntry struct {
	// Key is the resolved env-var key.
	Key string `json:"key"`
	// Value is the resolved value, masked unless --reveal was set.
	Value string `json:"value"`
	// Source is the file that provided the resolved value.
	Source string `json:"source"`
	// SourceKey is the original key in the source file.
	SourceKey string `json:"sourceKey"`
	// Shadowed lists the files overridden by the resolved value.
	Shadowed []string `json:"shadowed,omitempty"`
}

// -------------------------------------------------------------------------------------

// renderParams bundles everything render needs: the output sink, the structured
// result, the chosen output format, and whether values are revealed.
type renderParams struct {
	// Writer is the output sink to render to.
	Writer io.Writer
	// Result is the structured data to render.
	Result actionResult
	// Format selects the output format ("json" or the default table).
	Format string
	// Reveal shows plaintext values instead of masking them.
	Reveal bool
}

// -------------------------------------------------------------------------------------

// render writes the result to p.Writer in the requested format ("json" or the
// default aligned table), masking values unless reveal is set.
func render(p *renderParams) error {
	masked := maskResult(p.Result, p.Reveal)
	if strings.EqualFold(p.Format, "json") {
		return renderJSON(p.Writer, masked)
	}
	return renderTable(p.Writer, masked)
}

// -------------------------------------------------------------------------------------

// maskResult returns a copy of res with values redacted unless reveal is set.
// Empty values stay empty.
func maskResult(res actionResult, reveal bool) actionResult {
	if reveal {
		return res
	}
	entries := make([]actionResultEntry, len(res.Entries))
	for i, e := range res.Entries {
		if e.Value != "" {
			e.Value = redacted
		}
		entries[i] = e
	}
	return actionResult{Entries: entries}
}

// -------------------------------------------------------------------------------------

// renderJSON writes the entries as a JSON array.
func renderJSON(w io.Writer, res actionResult) error {
	out := make([]jsonEntry, 0, len(res.Entries))
	for _, e := range res.Entries {
		out = append(out, jsonEntry(e))
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// -------------------------------------------------------------------------------------

// renderTable writes the entries as an aligned KEY/VALUE/SOURCE table.
func renderTable(w io.Writer, res actionResult) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE"); err != nil {
		return err
	}
	for _, e := range res.Entries {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Key, e.Value, e.Source); err != nil {
			return err
		}
	}
	return tw.Flush()
}
