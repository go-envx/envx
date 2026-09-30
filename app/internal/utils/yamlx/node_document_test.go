package yamlx_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/utils/yamlx"
	"gopkg.in/yaml.v3"
)

func newDocumentNode(roots ...*yaml.Node) *yaml.Node {
	return &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: roots,
	}
}

func TestEnsureDocumentMapping(t *testing.T) {
	t.Parallel()

	t.Run("nil doc returns error", func(t *testing.T) {
		t.Parallel()
		root, err := yamlx.EnsureDocumentMapping(nil)
		if err == nil || root != nil {
			t.Errorf("got (%#v, %v), want error", root, err)
		}
	})

	t.Run("zero kind initialized to mapping", func(t *testing.T) {
		t.Parallel()
		var doc yaml.Node
		root, err := yamlx.EnsureDocumentMapping(&doc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
			t.Fatalf("doc = %#v, want DocumentNode with 1 child", doc)
		}
		if root != doc.Content[0] || root.Kind != yaml.MappingNode {
			t.Errorf("root = %#v, want MappingNode", root)
		}
	})

	t.Run("empty document node initialized to mapping", func(t *testing.T) {
		t.Parallel()
		doc := &yaml.Node{Kind: yaml.DocumentNode}
		root, err := yamlx.EnsureDocumentMapping(doc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(doc.Content) != 1 || root != doc.Content[0] {
			t.Fatalf("unexpected content: %#v", doc.Content)
		}
		if root.Kind != yaml.MappingNode {
			t.Errorf("root.Kind = %v, want MappingNode", root.Kind)
		}
	})

	t.Run("valid document mapping returned", func(t *testing.T) {
		t.Parallel()
		mapping := yamlx.NewMappingNode()
		doc := newDocumentNode(mapping)
		root, err := yamlx.EnsureDocumentMapping(doc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root != mapping {
			t.Errorf("got root %#v, want %#v", root, mapping)
		}
	})

	t.Run("multiple document roots rejected", func(t *testing.T) {
		t.Parallel()
		doc := newDocumentNode(yamlx.NewMappingNode(), yamlx.NewMappingNode())
		root, err := yamlx.EnsureDocumentMapping(doc)
		if err == nil || root != nil {
			t.Errorf("got (%#v, %v), want error for multi-doc", root, err)
		}
	})

	t.Run("non-mapping root rejected", func(t *testing.T) {
		t.Parallel()
		scalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "hello"}
		doc := newDocumentNode(scalar)
		root, err := yamlx.EnsureDocumentMapping(doc)
		if err == nil || root != nil {
			t.Errorf("got (%#v, %v), want error for scalar root", root, err)
		}
	})
}
