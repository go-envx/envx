package yamlx

import (
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// MappingEntry represents a key-value entry within a YAML mapping node.
type MappingEntry struct {
	// Index is the 0-based offset of the key in mapping.Content.
	Index int
	// Key is the matched key string (preserving original document casing).
	Key string
	// KeyNode is the scalar node representing the entry's key.
	KeyNode *yaml.Node
	// ValueNode is the node representing the entry's value.
	ValueNode *yaml.Node
}

// IsEmpty reports whether the entry represents an uninitialized or empty entry.
func (e MappingEntry) IsEmpty() bool {
	return e.KeyNode == nil && e.ValueNode == nil
}

// MatchOption configures entry matching behavior.
type MatchOption int

const (
	// IgnoreCase enables case-insensitive key matching.
	IgnoreCase MatchOption = iota + 1
)

// IsMappingNode reports whether node is a non-nil mapping node.
func IsMappingNode(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.MappingNode
}

// NewMappingNode creates a new YAML block mapping node.
func NewMappingNode() *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
}

// AppendMappingEntry appends a string key and value node pair to a mapping node.
// It panics if mapping is nil or not a MappingNode.
func AppendMappingEntry(mapping *yaml.Node, key string, value *yaml.Node) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		panic("yamlx: cannot append to non-mapping node")
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: key,
		},
		value,
	)
}

// DeleteMappingEntry finds and removes an entry by key, returning true if removed.
// It panics if mapping is nil or not a MappingNode.
func DeleteMappingEntry(
	mapping *yaml.Node,
	key string,
	opts ...MatchOption,
) bool {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		panic("yamlx: cannot delete from non-mapping node")
	}
	entry, found := FindMappingEntry(mapping, key, opts...)
	if !found {
		return false
	}
	deleteMappingEntryAtIndex(mapping, entry.Index)
	return true
}

func deleteMappingEntryAtIndex(mapping *yaml.Node, index int) {
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

// FindMappingEntry searches mapping for key (exact by default; case-insensitive
// with IgnoreCase). When mapping is nil or not a MappingNode, it returns
// (MappingEntry{}, false).
func FindMappingEntry(
	mapping *yaml.Node,
	key string,
	opts ...MatchOption,
) (MappingEntry, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return MappingEntry{Index: -1}, false
	}
	ignoreCase := slices.Contains(opts, IgnoreCase)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		name := keyNode.Value
		if name == key || (ignoreCase && strings.EqualFold(name, key)) {
			return MappingEntry{
				Index:     i,
				Key:       name,
				KeyNode:   keyNode,
				ValueNode: mapping.Content[i+1],
			}, true
		}
	}
	return MappingEntry{Index: -1}, false
}

// SetMappingEntry sets key and value in mapping. If key exists, it replaces or updates
// value (preserving key comments and copying existing value comments if replacement
// has none); if absent, it appends a new key-value pair.
// It panics if mapping is nil or not a MappingNode.
func SetMappingEntry(
	mapping *yaml.Node,
	key string,
	value *yaml.Node,
	opts ...MatchOption,
) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		panic("yamlx: cannot set entry in non-mapping node")
	}

	entry, found := FindMappingEntry(mapping, key, opts...)
	if !found {
		AppendMappingEntry(mapping, key, value)
		return
	}

	// Preserve comments if replacement has none
	if value != nil && entry.ValueNode != nil {
		if value.HeadComment == "" && value.LineComment == "" && value.FootComment == "" {
			value.HeadComment = entry.ValueNode.HeadComment
			value.LineComment = entry.ValueNode.LineComment
			value.FootComment = entry.ValueNode.FootComment
		}
	}

	mapping.Content[entry.Index+1] = value
}
