package filestore

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/lib/yamlx"
)

// defaultIndent is used when a document has no nested mapping to measure.
const defaultIndent = 2

// setInDocument writes value at the dotted key of a YAML document, preserving
// comments, order, and blank lines. Empty data starts a new document.
func setInDocument(data []byte, key, value string) ([]byte, error) {
	doc := new(yaml.Node)
	if err := yaml.Unmarshal(data, doc); err != nil {
		return nil, fmt.Errorf("parsing: %w", err)
	}
	indent := defaultIndent
	if n, ok := yamlx.IndentLevel(doc); ok {
		indent = n
	}
	root, err := documentRoot(doc)
	if err != nil {
		return nil, err
	}
	if err := setNestedKey(root, strings.Split(key, "."), value); err != nil {
		return nil, err
	}
	out, err := yamlx.Marshal(doc, indent)
	if err != nil {
		return nil, fmt.Errorf("marshaling: %w", err)
	}
	return yamlx.PreserveBlankLines(data, out), nil
}

// documentRoot returns the root mapping, seeding one for an empty document and
// promoting an explicit null root so its comments survive.
func documentRoot(doc *yaml.Node) (*yaml.Node, error) {
	if len(doc.Content) == 0 {
		doc.Kind = yaml.DocumentNode
		root := &yaml.Node{Kind: yaml.MappingNode}
		doc.Content = []*yaml.Node{root}
		return root, nil
	}
	root := doc.Content[0]
	switch {
	case root.Kind == yaml.MappingNode:
		return root, nil
	case root.Kind == yaml.ScalarNode && root.Tag == "!!null":
		root.Kind = yaml.MappingNode
		root.Tag = ""
		root.Value = ""
		return root, nil
	default:
		return nil, errors.New("document root is not a mapping")
	}
}

// setNestedKey walks parts through nested mappings, creating missing ones and
// refining scalars into mappings, then writes value at the last part. Lists and
// mappings are never overwritten; an existing scalar is updated in place.
func setNestedKey(node *yaml.Node, parts []string, value string) error {
	for i, part := range parts[:len(parts)-1] {
		entry, _ := yamlx.FindMappingEntry(node, part)
		child := entry.ValueNode
		switch {
		case child == nil:
			child = &yaml.Node{Kind: yaml.MappingNode}
			appendPair(node, part, child)
		case child.Kind == yaml.MappingNode:
			// Existing branch: descend without touching its entries.
		case child.Kind == yaml.SequenceNode:
			return fmt.Errorf(
				"%q is a list; refusing to overwrite it", strings.Join(parts[:i+1], "."),
			)
		default:
			*child = yaml.Node{Kind: yaml.MappingNode}
		}
		node = child
	}

	last := parts[len(parts)-1]
	entry, found := yamlx.FindMappingEntry(node, last)
	if found && entry.ValueNode != nil {
		v := entry.ValueNode
		if v.Kind == yaml.SequenceNode {
			return fmt.Errorf("%q is a list; refusing to overwrite it", strings.Join(parts, "."))
		}
		if v.Kind == yaml.MappingNode {
			return fmt.Errorf(
				"%q is a mapping; refusing to overwrite it", strings.Join(parts, "."),
			)
		}
		yamlx.SetScalarNode(v, value)
		return nil
	}
	leaf := new(yaml.Node)
	yamlx.SetScalarNode(leaf, value)
	appendPair(node, last, leaf)
	return nil
}

// appendPair appends a key/value entry to a mapping.
func appendPair(node *yaml.Node, key string, value *yaml.Node) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	node.Content = append(node.Content, keyNode, value)
}
