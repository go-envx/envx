package env

// resolveLeaf dereferences each item of a non-opaque leaf, preserving list
// boundaries. A nil resolver is identity.
func resolveLeaf(
	leaf leafValue, resolver ValueResolver, environment string,
) (leafValue, error) {
	if leaf.opaque || resolver == nil {
		return leaf, nil
	}
	resolved := leafValue{items: make([]string, len(leaf.items)), list: leaf.list}
	for i, item := range leaf.items {
		text, err := resolver.Resolve(item, environment)
		if err != nil {
			return leafValue{}, err
		}
		resolved.items[i] = text
	}
	return resolved, nil
}
