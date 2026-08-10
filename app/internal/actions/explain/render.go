package explain

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// jsonEntry is the exported, tagged view of an entry used for JSON output
// (entry's own fields are unexported).
type jsonEntry struct {
	// Key is the resolved env-var key.
	Key string `json:"key"`
	// Value is the resolved value.
	Value string `json:"value"`
	// Source is the file that provided the resolved value.
	Source string `json:"source"`
	// SourceKey is the original key in the source file.
	SourceKey string `json:"sourceKey"`
	// Shadowed lists the files overridden by the resolved value.
	Shadowed []string `json:"shadowed,omitempty"`
}

// renderParams bundles everything render needs: the output sink, the structured
// result, and the chosen output format.
type renderParams struct {
	// Writer is the output sink to render to.
	Writer io.Writer
	// Result is the structured data to render.
	Result actionResult
	// Format selects the output format ("json" or the default table).
	Format string
}

// render writes the result to p.Writer in the requested format ("json" or the
// default aligned table). An unrecognized format is rejected so a typo like
// --output=jsonn fails loudly.
func render(p *renderParams) error {
	switch p.Format {
	case "", "table":
		return renderTable(p.Writer, p.Result)
	case "json":
		return renderJSON(p.Writer, p.Result)
	default:
		return fmt.Errorf("invalid output format %q (want table or json)", p.Format)
	}
}

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
