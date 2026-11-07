package yamlx

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

var (
	// ErrDuplicateKey indicates a mapping node contains duplicate keys.
	ErrDuplicateKey = errors.New("yamlx: duplicate mapping key")

	// ErrTrailingDocument indicates additional YAML documents were found after the first.
	ErrTrailingDocument = errors.New("yamlx: trailing document")
)

// Parse decodes YAML data into a *yaml.Node AST.
// It rejects documents with duplicate mapping keys and streams containing
// trailing documents.
func Parse(data []byte) (*yaml.Node, error) {
	if len(data) == 0 {
		return &yaml.Node{Kind: yaml.DocumentNode}, nil
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return &yaml.Node{Kind: yaml.DocumentNode}, nil
		}
		return nil, fmt.Errorf("yamlx: %w", err)
	}

	var trailing yaml.Node
	if err := dec.Decode(&trailing); err == nil {
		return nil, ErrTrailingDocument
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("yamlx: %w", err)
	}

	if err := checkDuplicateKeys(&doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

// checkDuplicateKeys recursively validates that no mapping node contains
// duplicate keys.
func checkDuplicateKeys(node *yaml.Node) error {
	if node == nil {
		return nil
	}

	if node.Kind == yaml.MappingNode {
		seen := make(map[string]int, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			if keyNode.Kind == yaml.ScalarNode {
				if line, ok := seen[keyNode.Value]; ok {
					if keyNode.Line > 0 && line > 0 {
						return fmt.Errorf(
							"%w %q on line %d (first defined on line %d)",
							ErrDuplicateKey, keyNode.Value, keyNode.Line, line,
						)
					}
					if keyNode.Line > 0 {
						return fmt.Errorf(
							"%w %q on line %d",
							ErrDuplicateKey, keyNode.Value, keyNode.Line,
						)
					}
					return fmt.Errorf("%w %q", ErrDuplicateKey, keyNode.Value)
				}
				seen[keyNode.Value] = keyNode.Line
			} else if err := checkDuplicateKeys(keyNode); err != nil {
				return err
			}

			if err := checkDuplicateKeys(valNode); err != nil {
				return err
			}
		}
		return nil
	}

	for _, child := range node.Content {
		if err := checkDuplicateKeys(child); err != nil {
			return err
		}
	}

	return nil
}
