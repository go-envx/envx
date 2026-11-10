package format

import (
	"encoding/json"
	"io"
)

// renderJSON writes a sorted, indented JSON object without HTML escaping.
func renderJSON(w io.Writer, items []item) error {
	object := make(map[string]string, len(items))
	for _, it := range items {
		object[it.key] = it.value
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(object)
}
