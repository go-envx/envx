package emit

import (
	"encoding/json"
	"io"
)

// renderJSON writes entries as a single JSON object of key/value strings,
// indented for readability and newline-terminated. json.Encoder sorts a map's
// keys, so the output stays deterministic. HTML escaping is disabled so values
// carrying <, >, or & are written literally rather than as \u escapes.
func renderJSON(w io.Writer, entries []Entry) error {
	object := make(map[string]string, len(entries))
	for _, e := range entries {
		object[e.Key] = e.Value
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(object)
}
