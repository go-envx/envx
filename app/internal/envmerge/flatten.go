package envmerge

import (
	"fmt"
	"strings"
)

// leafValue preserves whether a YAML leaf was a scalar or list while
// base, overlay, and namespace winners are selected. Its items remain unresolved
// until the merge is complete.
type leafValue struct {
	// items holds one scalar value or every scalar item from a list.
	items []string
	// list distinguishes a YAML list from a scalar, including an empty list.
	list bool
}

// flatten converts a nested map to flat KEY=VALUE pairs while preserving scalar
// and list structure. Key paths are joined with "_" and uppercased
// (postgres.username -> POSTGRES_USERNAME). It errors when two distinct paths
// collapse to the same flat key or a list contains a non-scalar item.
func flatten(m map[string]any) (map[string]leafValue, error) {
	result := make(map[string]leafValue)
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
			value, err := leafValueFromYAML(v, path)
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

// leafValueFromYAML converts one YAML scalar or list leaf into its mergeable form. A
// list item must be scalar because nested lists and mappings have no flat env-var
// representation.
func leafValueFromYAML(v any, path string) (leafValue, error) {
	list, ok := v.([]any)
	if !ok {
		return leafValue{items: []string{scalarString(v)}}, nil
	}

	items := make([]string, len(list))
	for i, item := range list {
		if !isScalar(item) {
			return leafValue{}, fmt.Errorf(
				"list at %q has a non-scalar item; cannot flatten to an env var",
				path,
			)
		}
		items[i] = scalarString(item)
	}
	return leafValue{items: items, list: true}, nil
}

// renderLeafValue converts one resolved winning value to its final env-var
// string. Lists are joined with delimiter; an item containing that delimiter is
// rejected without including the potentially secret value in the error.
func renderLeafValue(
	value leafValue, path, delimiter string,
) (string, error) {
	if !value.list {
		return value.items[0], nil
	}

	for i, item := range value.items {
		if delimiter != "" && strings.Contains(item, delimiter) {
			return "", fmt.Errorf(
				"list item %d at %q contains the delimiter %q", i+1, path, delimiter,
			)
		}
	}
	return strings.Join(value.items, delimiter), nil
}

// literalValue renders one winning leaf to its display string without resolving
// references: a scalar is its single item and a list is joined with delimiter.
func literalValue(value leafValue, delimiter string) string {
	if !value.list {
		return value.items[0]
	}
	return strings.Join(value.items, delimiter)
}

// scalarString renders a scalar leaf value to its string form, mapping a nil
// leaf (a bare "key:") to "" rather than the "<nil>" fmt would produce.
func scalarString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

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

// toEnvKey converts a dotted nested path (postgres.username) to its env-var
// name (POSTGRES_USERNAME). Dots and hyphens become underscores.
func toEnvKey(path string) string {
	s := strings.ReplaceAll(path, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return strings.ToUpper(s)
}

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
