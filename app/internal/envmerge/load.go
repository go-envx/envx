package envmerge

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/pkg/file"
)

// -------------------------------------------------------------------------------------

// loadYAML reads and unmarshals a YAML file into a generic map. It returns a
// wrapped os.ErrNotExist when the file is missing so callers can distinguish
// "missing" from "malformed".
func loadYAML(path string) (map[string]any, error) {
	data, err := file.Read(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if m == nil {
		m = make(map[string]any)
	}
	return m, nil
}

// -------------------------------------------------------------------------------------

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
