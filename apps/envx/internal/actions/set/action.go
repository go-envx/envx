package set

import "strings"

// -------------------------------------------------------------------------------------
// actionParams are the inputs to the set action: the include path identifying
// the target overlay, the (possibly dotted) key, and the value to write.
type actionParams struct {
	IncludePath string
	Key         string
	Value       string
}

// -------------------------------------------------------------------------------------
// apply is the pure kernel: it returns doc with the key set, creating doc when
// nil. Plain data in, plain data out — no file I/O.
func apply(doc map[string]any, p actionParams) map[string]any {
	if doc == nil {
		doc = make(map[string]any)
	}
	setNestedKey(doc, p.Key, p.Value)
	return doc
}

// -------------------------------------------------------------------------------------
// setNestedKey sets value at a dot-separated key path within data, creating
// intermediate maps as needed and overwriting any non-map node that blocks the
// path.
func setNestedKey(data map[string]any, key, value string) {
	parts := strings.Split(key, ".")
	current := data
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}
