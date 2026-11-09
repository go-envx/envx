package pack

import (
	"fmt"
	"slices"
)

// selectDeclared returns the declared items named in requested, in declared
// order, failing with notDeclared for an unknown name. An empty request
// selects every item.
func selectDeclared[T any](
	declared []T, requested []string, name func(T) string, notDeclared error,
) ([]T, error) {
	if len(requested) == 0 {
		return slices.Clone(declared), nil
	}
	for _, want := range requested {
		if !slices.ContainsFunc(declared, func(item T) bool { return name(item) == want }) {
			return nil, fmt.Errorf("%w: %q", notDeclared, want)
		}
	}
	return slices.DeleteFunc(slices.Clone(declared), func(item T) bool {
		return !slices.Contains(requested, name(item))
	}), nil
}
