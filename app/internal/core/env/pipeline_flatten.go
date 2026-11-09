package env

import (
	"fmt"
	"strings"
)

// leafValue is one unresolved YAML leaf, preserving scalar versus list shape.
type leafValue struct {
	// items holds one scalar value or every item of a list.
	items []string
	// list distinguishes a YAML list, including an empty one, from a scalar.
	list bool
	// opaque marks a host value that bypasses resolution and substitution.
	opaque bool
}

// flatten converts a nested map to flat variable leaves and maps each key back
// to its dotted path. Paths are joined with "_" and uppercased; two paths
// colliding on one key or a non-scalar list item is an error.
func flatten(m map[string]any) (
	leaves map[string]leafValue, paths map[string]string, err error,
) {
	leaves = make(map[string]leafValue)
	paths = make(map[string]string)

	var walk func(prefix string, node map[string]any) error
	walk = func(prefix string, node map[string]any) error {
		for k, v := range node {
			path := joinPath(prefix, k)
			if sub, ok := toMap(v); ok {
				if err := walk(path, sub); err != nil {
					return err
				}
				continue
			}
			flatKey := toEnvKey(path)
			if existing, collision := paths[flatKey]; collision {
				// Order the paths so the message is independent of map iteration.
				first, second := min(existing, path), max(existing, path)
				return fmt.Errorf(
					"%w: %q and %q both produce key %q",
					ErrFlattenCollision, first, second, flatKey,
				)
			}
			paths[flatKey] = path
			leaf, err := leafFromYAML(v, path)
			if err != nil {
				return err
			}
			leaves[flatKey] = leaf
		}
		return nil
	}

	if err := walk("", m); err != nil {
		return nil, nil, err
	}
	return leaves, paths, nil
}

// leafFromYAML converts a scalar or list leaf. List items must be scalars.
func leafFromYAML(v any, path string) (leafValue, error) {
	list, ok := v.([]any)
	if !ok {
		return leafValue{items: []string{scalarString(v)}}, nil
	}
	items := make([]string, len(list))
	for i, item := range list {
		if !isScalar(item) {
			return leafValue{}, fmt.Errorf(
				"list at %q has a non-scalar item; cannot flatten to an env var", path,
			)
		}
		items[i] = scalarString(item)
	}
	return leafValue{items: items, list: true}, nil
}

// renderLeaf joins a resolved leaf into its variable value. A list item
// containing the delimiter is rejected without exposing the item.
func renderLeaf(leaf leafValue, path, delimiter string) (string, error) {
	if !leaf.list {
		return leaf.items[0], nil
	}
	for i, item := range leaf.items {
		if delimiter != "" && strings.Contains(item, delimiter) {
			return "", fmt.Errorf(
				"list item %d at %q contains the delimiter %q", i+1, path, delimiter,
			)
		}
	}
	return strings.Join(leaf.items, delimiter), nil
}

// literalOf joins a leaf's raw items without resolving or validating them.
func literalOf(leaf leafValue, delimiter string) string {
	if !leaf.list {
		return leaf.items[0]
	}
	return strings.Join(leaf.items, delimiter)
}

// toMap coerces v into map[string]any, including yaml's map[any]any form.
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

// scalarString renders a scalar, mapping nil (a bare "key:") to "".
func scalarString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// isScalar reports whether v is neither a mapping nor a sequence.
func isScalar(v any) bool {
	switch v.(type) {
	case map[string]any, map[any]any, []any:
		return false
	default:
		return true
	}
}

// joinPath appends key to a dotted path.
func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// toEnvKey converts a dotted path to a variable name; dots and hyphens become
// underscores.
func toEnvKey(path string) string {
	s := strings.ReplaceAll(path, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return strings.ToUpper(s)
}
