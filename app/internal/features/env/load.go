package env

import "fmt"

// toMap coerces a value into map[string]any, handling both the standard form
// and the map[any]any variant that yaml.v3 can produce.
func toMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[fmt.Sprintf("%v", k)] = val
		}
		return result, true
	}
	return nil, false
}
