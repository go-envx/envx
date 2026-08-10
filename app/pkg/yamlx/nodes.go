package yamlx

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// MappingEntry returns the value and index paired with key in a block mapping. When
// ignoreCase is true, the key comparison is case-insensitive. A nil node, a
// non-mapping node, or an absent key yields a nil node and index -1.
func MappingEntry(mapping *yaml.Node, key string, ignoreCase bool) (
	value *yaml.Node, index int,
) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, -1
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		name := mapping.Content[i].Value
		if name == key || (ignoreCase && strings.EqualFold(name, key)) {
			return mapping.Content[i+1], i
		}
	}
	return nil, -1
}

// RemoveMappingEntry deletes the key/value pair beginning at index from a block
// mapping, preserving the order of the remaining entries. It panics if mapping
// is invalid or index does not identify a valid key/value pair.
func RemoveMappingEntry(mapping *yaml.Node, index int) {
	invalid := mapping == nil ||
		mapping.Kind != yaml.MappingNode ||
		len(mapping.Content)%2 != 0 ||
		index < 0 ||
		index >= len(mapping.Content) ||
		index%2 != 0 ||
		index+1 >= len(mapping.Content)

	if invalid {
		panic("yamlx: invalid mapping entry index")
	}

	last := len(mapping.Content) - 2
	copy(mapping.Content[index:], mapping.Content[index+2:])
	mapping.Content[last] = nil
	mapping.Content[last+1] = nil
	mapping.Content = mapping.Content[:last]
}

// SetStringScalar rewrites node as a plain string scalar, clearing any prior
// style and children so the encoder can requote as needed. Only the value fields
// change, so an existing node keeps its comments and position.
func SetStringScalar(node *yaml.Node, value string) {
	node.Kind = yaml.ScalarNode
	node.Tag = "!!str"
	node.Style = 0
	node.Value = value
	node.Content = nil
}
