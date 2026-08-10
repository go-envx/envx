package store

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// mappingEntry describes the result of a mapping lookup.
type mappingEntry struct {
	index int
	key   string
	value *yaml.Node
	found bool
}

// getMappingEntry finds a mapping value by key while optionally matching names.
func getMappingEntry(mapping *yaml.Node, name string, ignoreCase bool) (
	mappingEntry, error,
) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return mappingEntry{index: -1}, errors.New("YAML value is not a mapping")
	}
	if len(mapping.Content)%2 != 0 {
		return mappingEntry{index: -1}, errors.New("mapping has an incomplete entry")
	}

	for i := 0; i < len(mapping.Content); i += 2 {
		key, err := getStringValue(mapping.Content[i], "mapping key")
		if err != nil {
			return mappingEntry{index: -1}, err
		}
		matches := key == name
		if ignoreCase {
			matches = strings.EqualFold(key, name)
		}
		if matches {
			return mappingEntry{
				index: i,
				key:   key,
				value: mapping.Content[i+1],
				found: true,
			}, nil
		}
	}
	return mappingEntry{index: -1}, nil
}

// getStringValue returns a YAML scalar's string content.
func getStringValue(node *yaml.Node, description string) (string, error) {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "", fmt.Errorf("%s must be a string", description)
	}
	return node.Value, nil
}

// newMappingNode creates an empty YAML mapping node.
func newMappingNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

// newStringNode creates a YAML string scalar node.
func newStringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

// appendMappingEntry appends a string key and value to mapping.
func appendMappingEntry(mapping *yaml.Node, key, value string) {
	appendMappingNode(mapping, key, newStringNode(value))
}

// appendMappingNode appends a string key and an existing value node to mapping.
func appendMappingNode(mapping *yaml.Node, key string, value *yaml.Node) {
	mapping.Content = append(mapping.Content, newStringNode(key), value)
}

// removeMappingEntry removes a key/value pair beginning at index.
func removeMappingEntry(mapping *yaml.Node, index int) {
	last := len(mapping.Content) - 2
	copy(mapping.Content[index:], mapping.Content[index+2:])
	mapping.Content[last] = nil
	mapping.Content[last+1] = nil
	mapping.Content = mapping.Content[:last]
}
