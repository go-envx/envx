package store

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-envx/envx/app/pkg/file"
	"github.com/go-envx/envx/app/pkg/yamlx"
	"gopkg.in/yaml.v3"
)

// Secret identifies one stored value by its group and key.
type Secret struct {
	// Group is the stored key-group name.
	Group string
	// Key is the entry name within Group.
	Key string
	// Value is the stored plaintext or ciphertext value.
	Value string
}

// Document is a validated mutable secrets document bound to one file path.
type Document struct {
	// path is the file this document will save to.
	path string
	// root is the YAML node tree retained for comments and ordering.
	root yaml.Node
}

// Open reads a secrets document and binds it to path. A missing file produces
// an empty document; malformed YAML or an invalid known document field fails.
func Open(path string) (*Document, error) {
	data, err := file.Read(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newEmptyDocument(path), nil
		}
		return nil, fmt.Errorf("reading secrets %s: %w", path, err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing secrets %s: %w", path, err)
	}

	document := &Document{path: path, root: root}
	document.normalizeEmptyRoot()
	if err := document.validate(); err != nil {
		return nil, fmt.Errorf("validating secrets %s: %w", path, err)
	}
	return document, nil
}

// Save validates and atomically writes the document to its bound path. It
// preserves the document's own block indentation, applying defaultIndent only
// when the document has none of its own.
func (d *Document) Save(defaultIndent int) error {
	if d.path == "" {
		return errors.New("secrets document path is empty")
	}
	if err := d.validate(); err != nil {
		return fmt.Errorf("validating secrets: %w", err)
	}

	indent := defaultIndent
	if own, ok := yamlx.IndentLevel(&d.root); ok {
		indent = own
	}

	data, err := yamlx.Marshal(&d.root, indent)
	if err != nil {
		return fmt.Errorf("encoding secrets %s: %w", d.path, err)
	}
	if err := file.WriteAtomic(d.path, data); err != nil {
		return fmt.Errorf("writing secrets %s: %w", d.path, err)
	}
	return nil
}

// newEmptyDocument creates the canonical empty YAML document.
func newEmptyDocument(path string) *Document {
	return &Document{
		path: path,
		root: yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				newMappingNode(),
			},
		},
	}
}

// normalizeEmptyRoot turns an empty YAML stream into an editable mapping.
func (d *Document) normalizeEmptyRoot() {
	if d.root.Kind == 0 ||
		(d.root.Kind == yaml.DocumentNode && len(d.root.Content) == 0) {
		d.root = newEmptyDocument(d.path).root
	}
}

// topLevel returns the document's top-level mapping.
func (d *Document) topLevel() (*yaml.Node, error) {
	d.normalizeEmptyRoot()
	if d.root.Kind != yaml.DocumentNode || len(d.root.Content) != 1 {
		return nil, errors.New("secrets document must contain one YAML document")
	}
	root := d.root.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("secrets document must be a mapping")
	}
	return root, nil
}
