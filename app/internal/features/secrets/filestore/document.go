package filestore

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/utils/strictyaml"
	"github.com/go-envx/envx/app/internal/utils/yamlx"
	"gopkg.in/yaml.v3"
)

const (
	publicKeysField = "public-keys"
	secretsField    = "secrets"

	// defaultSuggestionThreshold is the largest edit distance for suggestion.
	defaultSuggestionThreshold = 3
)

// document is a validated mutable in-memory YAML representation of a secrets document.
type document struct {
	// root is the YAML node tree retained for comments and ordering.
	root yaml.Node
	// source is the original file content, retained to restore blank lines that
	// the node tree does not model.
	source []byte
}

// parseDocument parses raw YAML secrets data into an editable document,
// applying strict validation to reject unknown fields and malformed structures.
func parseDocument(data []byte) (*document, error) {
	if len(data) == 0 {
		var doc document
		_, _ = yamlx.EnsureDocumentMapping(&doc.root)
		return &doc, nil
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing secrets: %w", err)
	}

	if root.Kind != 0 {
		var schema secretsYAML
		dec := strictyaml.New(
			strictyaml.WithSchema[secretsYAML]("secrets field"),
			strictyaml.WithSuggestionThreshold(defaultSuggestionThreshold),
		)
		if err := dec.Decode(data, &schema); err != nil {
			return nil, fmt.Errorf("parsing secrets: %w", err)
		}
	}

	doc := &document{root: root, source: data}
	if _, err := yamlx.EnsureDocumentMapping(&doc.root); err != nil {
		return nil, fmt.Errorf("parsing secrets: %w", err)
	}
	if err := doc.validate(); err != nil {
		return nil, fmt.Errorf("validating secrets: %w", err)
	}
	return doc, nil
}

// encode serializes the document into formatted YAML bytes, preserving the
// document's own block indentation and blank lines from source.
func (d *document) encode(defaultIndent int) ([]byte, error) {
	if err := d.validate(); err != nil {
		return nil, fmt.Errorf("validating secrets: %w", err)
	}

	indent := defaultIndent
	if own, ok := yamlx.IndentLevel(&d.root); ok {
		indent = own
	}

	data, err := yamlx.Marshal(&d.root, indent)
	if err != nil {
		return nil, fmt.Errorf("encoding secrets: %w", err)
	}
	data = yamlx.PreserveBlankLines(d.source, data)
	return data, nil
}

// rootMapping returns the document's top-level mapping.
func (d *document) rootMapping() (*yaml.Node, error) {
	root, err := yamlx.EnsureDocumentMapping(&d.root)
	if err != nil {
		return nil, errors.New("secrets document must be a mapping")
	}
	return root, nil
}

// ensureFieldMapping returns a known top-level mapping, creating it when absent.
func (d *document) ensureFieldMapping(field string) (*yaml.Node, error) {
	root, err := d.rootMapping()
	if err != nil {
		return nil, err
	}
	entry, found := yamlx.FindMappingEntry(root, field)
	if found {
		if !yamlx.IsMappingNode(entry.ValueNode) {
			return nil, fmt.Errorf("%s must be a mapping", field)
		}
		return entry.ValueNode, nil
	}

	node := yamlx.NewMappingNode()
	yamlx.AppendMappingEntry(root, field, node)
	return node, nil
}

// expectString returns a YAML scalar's string content or an error if not a
// string scalar.
func (d *document) expectString(node *yaml.Node, description string) (string, error) {
	if !yamlx.IsScalarNode[string](node) {
		return "", fmt.Errorf("%s must be a string", description)
	}
	return node.Value, nil
}

// validate checks the known document fields and their contents.
func (d *document) validate() error {
	root, err := d.rootMapping()
	if err != nil {
		return err
	}

	seen := make(map[string]struct{})
	for i := 0; i < len(root.Content); i += 2 {
		if i+1 >= len(root.Content) {
			return errors.New("top-level mapping has an incomplete entry")
		}
		field, err := d.expectString(root.Content[i], "top-level key")
		if err != nil {
			return err
		}
		if _, exists := seen[field]; exists {
			return fmt.Errorf("duplicate top-level field %q", field)
		}
		seen[field] = struct{}{}

		switch field {
		case publicKeysField:
			if err := d.validatePublicKeys(root.Content[i+1]); err != nil {
				return err
			}
		case secretsField:
			if err := d.validateSecrets(root.Content[i+1]); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown top-level field %q", field)
		}
	}
	return nil
}
