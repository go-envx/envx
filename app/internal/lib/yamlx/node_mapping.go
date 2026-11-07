package yamlx

import (
	"errors"
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

// Lookup finds the value node associated with key in mapping.
// It returns (nil, false) if mapping is nil, not a mapping node,
// or does not contain key.
func Lookup(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	if mapping == nil {
		return nil, false
	}
	if mapping.Kind == yaml.DocumentNode && len(mapping.Content) > 0 {
		mapping = mapping.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1], true
		}
	}
	return nil, false
}

// Set sets key to node in mapping, replacing an existing entry's value or appending
// a new key-value pair. It returns an error if mapping is nil or not a mapping node,
// or if node is nil.
func Set(mapping *yaml.Node, key string, node *yaml.Node) error {
	if mapping == nil {
		return errors.New("yamlx: cannot set entry on nil node")
	}
	if mapping.Kind == 0 ||
		(mapping.Kind == yaml.DocumentNode && len(mapping.Content) == 0) {
		mapping.Kind = yaml.DocumentNode
		mapping.Content = []*yaml.Node{NewMappingNode()}
		mapping = mapping.Content[0]
	} else if mapping.Kind == yaml.DocumentNode {
		mapping = mapping.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return errors.New("yamlx: cannot set entry on non-mapping node")
	}
	if node == nil {
		return errors.New("yamlx: cannot set nil node value")
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			old := mapping.Content[i+1]
			if node.HeadComment == "" &&
				node.LineComment == "" &&
				node.FootComment == "" {
				node.HeadComment = old.HeadComment
				node.LineComment = old.LineComment
				node.FootComment = old.FootComment
			}
			mapping.Content[i+1] = node
			return nil
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: key,
		},
		node,
	)
	return nil
}

// Delete removes the entry matching key from mapping, returning true if found
// and removed. It returns false if mapping is nil, not a mapping node, or key
// is not present.
func Delete(mapping *yaml.Node, key string) bool {
	if mapping == nil {
		return false
	}
	if mapping.Kind == yaml.DocumentNode && len(mapping.Content) > 0 {
		mapping = mapping.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			deleteMappingEntryAtIndex(mapping, i)
			return true
		}
	}
	return false
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
