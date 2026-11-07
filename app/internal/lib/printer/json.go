package printer

import "encoding/json"

// WriteJSON writes value as indented JSON to standard output. It is never
// colored so the output stays machine-readable.
func (p *Printer) WriteJSON(value any) error {
	enc := json.NewEncoder(p.out)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
