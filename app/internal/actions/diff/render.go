package diff

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// -------------------------------------------------------------------------------------

// jsonChange is the exported, tagged view of a change used for JSON output.
type jsonChange struct {
	// Key is the env-var key that differs.
	Key string `json:"key"`
	// EnvA is the value under env-a (empty for additions).
	EnvA string `json:"env_a,omitempty"`
	// EnvB is the value under env-b (empty for removals).
	EnvB string `json:"env_b,omitempty"`
}

// -------------------------------------------------------------------------------------

// jsonResult is the exported, tagged view of the whole diff used for JSON
// output.
type jsonResult struct {
	// Added lists keys present only in env-b.
	Added []jsonChange `json:"added,omitempty"`
	// Removed lists keys present only in env-a.
	Removed []jsonChange `json:"removed,omitempty"`
	// Changed lists keys present in both with differing values.
	Changed []jsonChange `json:"changed,omitempty"`
}

// -------------------------------------------------------------------------------------

// renderParams bundles everything render needs: the output sink, the structured
// diff, and the chosen output format.
type renderParams struct {
	// Writer is the output sink to render to.
	Writer io.Writer
	// Result is the structured diff to render.
	Result actionResult
	// Format selects the output format ("json" or the default table).
	Format string
}

// -------------------------------------------------------------------------------------

// render writes the diff to p.Writer in the requested format. An unrecognized
// format is rejected so a typo like --output=jsonn fails loudly.
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

// -------------------------------------------------------------------------------------

// renderJSON writes the diff as an indented JSON object.
func renderJSON(w io.Writer, res actionResult) error {
	view := jsonResult{
		Added:   toJSONChanges(res.Added),
		Removed: toJSONChanges(res.Removed),
		Changed: toJSONChanges(res.Changed),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(view)
}

// -------------------------------------------------------------------------------------

// toJSONChanges converts internal changes to their tagged JSON view.
func toJSONChanges(in []actionResultChange) []jsonChange {
	out := make([]jsonChange, 0, len(in))
	for _, c := range in {
		out = append(out, jsonChange(c))
	}
	return out
}

// -------------------------------------------------------------------------------------

// renderTable writes the diff as aligned, sign-prefixed rows:
// (+ added, - removed, ~ changed)
func renderTable(w io.Writer, res actionResult) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, c := range res.Added {
		if _, err := fmt.Fprintf(tw, "+\t%s\t%s\n", c.Key, c.EnvB); err != nil {
			return err
		}
	}
	for _, c := range res.Removed {
		if _, err := fmt.Fprintf(tw, "-\t%s\t%s\n", c.Key, c.EnvA); err != nil {
			return err
		}
	}
	for _, c := range res.Changed {
		if _, err := fmt.Fprintf(
			tw, "~\t%s\t%s -> %s\n", c.Key, c.EnvA, c.EnvB,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}
