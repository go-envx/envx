package envmerge

import (
	"fmt"
	"strings"

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

// -------------------------------------------------------------------------------------

// flatten converts a nested map to flat KEY=VALUE pairs suitable for env vars.
// Key paths are joined with "_" and uppercased (postgres.username ->
// POSTGRES_USERNAME) and a list leaf is joined into a single delimiter-separated
// string. It returns an error when two distinct paths collapse to the same flat
// key (preventing silent data loss) or when a list cannot be rendered (see
// leafValue).
func flatten(m map[string]any, delimiter string) (map[string]string, error) {
	result := make(map[string]string)
	origins := make(map[string]string)

	var walk func(prefix string, node map[string]any) error
	walk = func(prefix string, node map[string]any) error {
		for k, v := range node {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			flatKey := toEnvKey(path)

			if sub, isMap := toMap(v); isMap {
				if err := walk(path, sub); err != nil {
					return err
				}
				continue
			}
			if existing, collision := origins[flatKey]; collision {
				// Order the two paths so the message is stable regardless of the
				// map iteration order that surfaced the collision.
				first, second := existing, path
				if first > second {
					first, second = second, first
				}
				return fmt.Errorf(
					"flatten collision: %q and %q both produce key %q",
					first, second, flatKey,
				)
			}
			origins[flatKey] = path
			value, err := leafValue(v, path, delimiter)
			if err != nil {
				return err
			}
			result[flatKey] = value
		}
		return nil
	}

	if err := walk("", m); err != nil {
		return nil, err
	}
	return result, nil
}

// -------------------------------------------------------------------------------------

// leafValue renders a flattened leaf to its env-var string form. A scalar yields
// itself and a nil leaf (a bare "key:") yields "", rather than the "<nil>" fmt
// would otherwise produce. A list is joined into a single delimiter-separated
// string so a value can be authored as a YAML sequence; it errors when an item
// is itself a list or mapping (which has no flat env-var form) or contains the
// delimiter (which would make the joined value impossible to split back).
func leafValue(v any, path, delimiter string) (string, error) {
	list, ok := v.([]any)
	if !ok {
		if v == nil {
			return "", nil
		}
		return fmt.Sprintf("%v", v), nil
	}

	items := make([]string, len(list))
	for i, item := range list {
		if !isScalar(item) {
			return "", fmt.Errorf(
				"list at %q has a non-scalar item; cannot flatten to an env var",
				path,
			)
		}
		s := ""
		if item != nil {
			s = fmt.Sprintf("%v", item)
		}
		if delimiter != "" && strings.Contains(s, delimiter) {
			return "", fmt.Errorf(
				"list item %q at %q contains the delimiter %q", s, path, delimiter,
			)
		}
		items[i] = s
	}
	return strings.Join(items, delimiter), nil
}

// -------------------------------------------------------------------------------------

// isScalar reports whether v is a YAML scalar leaf — neither a mapping nor a
// sequence — and so can be rendered directly into an env-var value.
func isScalar(v any) bool {
	switch v.(type) {
	case map[string]any, map[any]any, []any:
		return false
	default:
		return true
	}
}

// -------------------------------------------------------------------------------------

// toEnvKey converts a dotted nested path (postgres.username) to its env-var
// name (POSTGRES_USERNAME). Dots and hyphens become underscores.
func toEnvKey(path string) string {
	s := strings.ReplaceAll(path, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return strings.ToUpper(s)
}

// -------------------------------------------------------------------------------------

// flattenKeys returns a lookup mapping each flat env key back to its original
// dotted path, used for origin tracking.
func flattenKeys(m map[string]any) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string)
	var walk func(prefix string, node map[string]any)
	walk = func(prefix string, node map[string]any) {
		for k, v := range node {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			if sub, isMap := toMap(v); isMap {
				walk(path, sub)
			} else {
				result[toEnvKey(path)] = path
			}
		}
	}
	walk("", m)
	return result
}

// -------------------------------------------------------------------------------------

// transformKey applies the prefix and suffix to a single key.
func transformKey(key, prefix, suffix string) string {
	if prefix != "" {
		key = prefix + "_" + key
	}
	if suffix != "" {
		key = key + "_" + suffix
	}
	return key
}
