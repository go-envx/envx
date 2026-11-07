package yamlx

import (
	"errors"

	"gopkg.in/yaml.v3"
)

// EnsureDocumentMapping returns the single top-level mapping node inside doc.
// If doc is uninitialized or represents an empty document stream, it normalizes
// doc in place into a DocumentNode containing a single empty MappingNode.
// An error is returned if doc is nil, contains multiple documents, or its root
// is not a mapping.
func EnsureDocumentMapping(doc *yaml.Node) (*yaml.Node, error) {
	if doc == nil {
		return nil, errors.New("yamlx: nil document node")
	}

	if doc.Kind == 0 || (doc.Kind == yaml.DocumentNode && len(doc.Content) == 0) {
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{NewMappingNode()}
		return doc.Content[0], nil
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return nil, errors.New("yamlx: document must contain exactly one root")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("yamlx: document root must be a mapping")
	}

	return root, nil
}
