package diff

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/printer"
	"github.com/go-envx/envx/app/internal/style"
)

// jsonChange is the exported, tagged view of a change used for JSON output.
type jsonChange struct {
	// Key is the env-var key that differs.
	Key string `json:"key"`
	// EnvA is the value under env-a (empty for additions).
	EnvA string `json:"env_a,omitempty"`
	// EnvB is the value under env-b (empty for removals).
	EnvB string `json:"env_b,omitempty"`
}

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

// renderParams bundles everything render needs: the printer, the structured
// diff, and the chosen output format.
type renderParams struct {
	// Printer is the styled output layer for the table and JSON.
	Printer *printer.Printer
	// Result is the structured diff to render.
	Result actionResult
	// Format selects the output format ("json" or the default table).
	Format string
}

// render writes the diff in the requested format. An unrecognized format is
// rejected so a typo like --output=jsonn fails loudly.
func render(p *renderParams) error {
	switch p.Format {
	case "", "table":
		return renderTable(p.Printer, p.Result)
	case "json":
		return renderJSON(p.Printer, p.Result)
	default:
		return fmt.Errorf("invalid output format %q (want table or json)", p.Format)
	}
}

// renderJSON writes the diff as an indented JSON object.
func renderJSON(p *printer.Printer, res actionResult) error {
	return p.WriteJSON(jsonResult{
		Added:   toJSONChanges(res.Added),
		Removed: toJSONChanges(res.Removed),
		Changed: toJSONChanges(res.Changed),
	})
}

// toJSONChanges converts internal changes to their tagged JSON view.
func toJSONChanges(in []actionResultChange) []jsonChange {
	out := make([]jsonChange, 0, len(in))
	for _, c := range in {
		out = append(out, jsonChange(c))
	}
	return out
}

// renderTable writes the diff as a headerless, sign-prefixed table following the
// conventional diff palette: additions green (+), removals red (-), and changes
// yellow (~). A run with no differences prints nothing.
func renderTable(p *printer.Printer, res actionResult) error {
	rows := make([][]printer.Cell, 0, len(res.Added)+len(res.Removed)+len(res.Changed))
	for _, c := range res.Added {
		rows = append(rows, changeRow(style.ColorGreen, "+", c.Key, c.EnvB))
	}
	for _, c := range res.Removed {
		rows = append(rows, changeRow(style.ColorRed, "-", c.Key, c.EnvA))
	}
	for _, c := range res.Changed {
		rows = append(rows, changeRow(style.ColorYellow, "~", c.Key, c.EnvA+" -> "+c.EnvB))
	}
	return p.WriteTable(printer.Table{Rows: rows})
}

// changeRow builds a sign/key/value row whose cells all carry color so the whole
// line reads in the diff palette.
func changeRow(color style.Color, sign, key, value string) []printer.Cell {
	return []printer.Cell{
		{Text: sign, Color: color},
		{Text: key, Color: color},
		{Text: value, Color: color},
	}
}
