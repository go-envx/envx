package yamlx_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/lib/yamlx"
	"gopkg.in/yaml.v3"
)

func TestIsMappingNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		node *yaml.Node
		want bool
	}{
		{
			name: "nil node",
			node: nil,
			want: false,
		},
		{
			name: "mapping node",
			node: &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"},
			want: true,
		},
		{
			name: "scalar node",
			node: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "hello"},
			want: false,
		},
		{
			name: "sequence node",
			node: &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := yamlx.IsMappingNode(tc.node); got != tc.want {
				t.Errorf("IsMappingNode() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNewMappingNode(t *testing.T) {
	t.Parallel()

	node := yamlx.NewMappingNode()
	if node == nil {
		t.Fatal("NewMappingNode() returned nil")
	}
	if node.Kind != yaml.MappingNode || node.Tag != "!!map" {
		t.Errorf("NewMappingNode() = %#v", node)
	}
}

func TestAppendMappingEntry(t *testing.T) {
	t.Parallel()

	mapping := yamlx.NewMappingNode()
	child := yamlx.NewMappingNode()
	yamlx.AppendMappingEntry(mapping, "key", child)

	if len(mapping.Content) != 2 {
		t.Fatalf("len(mapping.Content) = %d, want 2", len(mapping.Content))
	}
	keyNode := mapping.Content[0]
	if keyNode.Value != "key" || keyNode.Kind != yaml.ScalarNode ||
		keyNode.Tag != "!!str" {
		t.Errorf("key node = %#v", keyNode)
	}
	if mapping.Content[1] != child {
		t.Errorf("value node = %#v, want %#v", mapping.Content[1], child)
	}

	assertPanics(t, func() {
		yamlx.AppendMappingEntry(nil, "key", child)
	})
	assertPanics(t, func() {
		yamlx.AppendMappingEntry(&yaml.Node{Kind: yaml.ScalarNode}, "key", child)
	})
}

func TestFindMappingEntry(t *testing.T) {
	t.Parallel()

	mapping := testMapping("First", "one", "Production", "two")
	tests := []struct {
		name      string
		key       string
		opts      []yamlx.MatchOption
		wantFound bool
		wantKey   string
		wantValue string
		wantIndex int
	}{
		{
			name:      "exact first entry",
			key:       "First",
			wantFound: true,
			wantKey:   "First",
			wantValue: "one",
			wantIndex: 0,
		},
		{
			name:      "case-sensitive mismatch",
			key:       "production",
			wantFound: false,
			wantIndex: -1,
		},
		{
			name:      "case-insensitive second entry",
			key:       "production",
			opts:      []yamlx.MatchOption{yamlx.IgnoreCase},
			wantFound: true,
			wantKey:   "Production",
			wantValue: "two",
			wantIndex: 2,
		},
		{
			name:      "missing",
			key:       "missing",
			wantFound: false,
			wantIndex: -1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, found := yamlx.FindMappingEntry(mapping, test.key, test.opts...)
			if found != test.wantFound {
				t.Fatalf("FindMappingEntry() found = %v, want %v", found, test.wantFound)
			}
			if entry.Index != test.wantIndex {
				t.Fatalf("FindMappingEntry() index = %d, want %d", entry.Index, test.wantIndex)
			}
			if !test.wantFound {
				if entry.KeyNode != nil || entry.ValueNode != nil {
					t.Fatalf("FindMappingEntry() entry = %#v, want empty", entry)
				}
				return
			}
			if entry.Key != test.wantKey {
				t.Errorf("FindMappingEntry() key = %q, want %q", entry.Key, test.wantKey)
			}
			if entry.KeyNode == nil || entry.KeyNode.Value != test.wantKey {
				t.Errorf("FindMappingEntry() keyNode = %#v", entry.KeyNode)
			}
			if entry.ValueNode == nil || entry.ValueNode.Value != test.wantValue {
				t.Errorf("FindMappingEntry() value = %#v, want %q", entry.ValueNode, test.wantValue)
			}
		})
	}
}

func TestFindMappingEntryReturnsNotFoundForInvalidNodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		node *yaml.Node
	}{
		{name: "nil", node: nil},
		{name: "scalar", node: &yaml.Node{Kind: yaml.ScalarNode}},
		{name: "sequence", node: &yaml.Node{Kind: yaml.SequenceNode}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, found := yamlx.FindMappingEntry(test.node, "key")
			if found || entry.KeyNode != nil || entry.ValueNode != nil || entry.Index != -1 {
				t.Errorf("FindMappingEntry() = (%#v, %v), want empty", entry, found)
			}
		})
	}
}

func TestDeleteMappingEntry(t *testing.T) {
	t.Parallel()

	mapping := testMapping("First", "one", "Second", "two", "Third", "three")

	// Delete non-existent
	if yamlx.DeleteMappingEntry(mapping, "nonexistent") {
		t.Error("DeleteMappingEntry returned true for nonexistent key")
	}

	// Delete Second with IgnoreCase
	if !yamlx.DeleteMappingEntry(mapping, "second", yamlx.IgnoreCase) {
		t.Fatal("DeleteMappingEntry failed to delete 'second'")
	}

	if len(mapping.Content) != 4 {
		t.Fatalf("mapping.Content len = %d, want 4", len(mapping.Content))
	}
	if mapping.Content[0].Value != "First" || mapping.Content[2].Value != "Third" {
		t.Errorf("unexpected content order after delete: %#v", mapping.Content)
	}

	// Delete First
	if !yamlx.DeleteMappingEntry(mapping, "First") {
		t.Fatal("DeleteMappingEntry failed to delete 'First'")
	}
	if len(mapping.Content) != 2 || mapping.Content[0].Value != "Third" {
		t.Errorf("unexpected content after second delete: %#v", mapping.Content)
	}

	// Delete Third
	if !yamlx.DeleteMappingEntry(mapping, "Third") {
		t.Fatal("DeleteMappingEntry failed to delete 'Third'")
	}
	if len(mapping.Content) != 0 {
		t.Errorf("unexpected content after deleting all entries: %#v", mapping.Content)
	}

	// Panics on invalid node
	assertPanics(t, func() {
		yamlx.DeleteMappingEntry(nil, "key")
	})
	assertPanics(t, func() {
		yamlx.DeleteMappingEntry(&yaml.Node{Kind: yaml.ScalarNode}, "key")
	})
}

func TestLookup(t *testing.T) {
	t.Parallel()

	mapping := testMapping("alpha", "1", "beta", "2")

	// Found
	val, ok := yamlx.Lookup(mapping, "alpha")
	if !ok || val == nil || val.Value != "1" {
		t.Errorf("Lookup(alpha) = (%v, %v), want (node(1), true)", val, ok)
	}

	// Missing
	val, ok = yamlx.Lookup(mapping, "missing")
	if ok || val != nil {
		t.Errorf("Lookup(missing) = (%v, %v), want (nil, false)", val, ok)
	}

	// Nil node
	val, ok = yamlx.Lookup(nil, "alpha")
	if ok || val != nil {
		t.Errorf("Lookup(nil) = (%v, %v), want (nil, false)", val, ok)
	}

	// Non-mapping node
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "hello"}
	val, ok = yamlx.Lookup(scalar, "alpha")
	if ok || val != nil {
		t.Errorf("Lookup(scalar) = (%v, %v), want (nil, false)", val, ok)
	}

	// Document containing mapping
	doc := newDocumentNode(mapping)
	val, ok = yamlx.Lookup(doc, "beta")
	if !ok || val == nil || val.Value != "2" {
		t.Errorf("Lookup(doc, beta) = (%v, %v), want (node(2), true)", val, ok)
	}
}

func TestSet(t *testing.T) {
	t.Parallel()

	t.Run("replace existing key", func(t *testing.T) {
		t.Parallel()
		mapping := testMapping("a", "1", "b", "2")
		newVal := yamlx.NewScalarNode("updated")

		err := yamlx.Set(mapping, "a", newVal)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, ok := yamlx.Lookup(mapping, "a")
		if !ok || got != newVal || got.Value != "updated" {
			t.Errorf("Lookup(a) = %v, want updated node", got)
		}
		// b should still exist and order preserved
		if len(mapping.Content) != 4 ||
			mapping.Content[0].Value != "a" ||
			mapping.Content[2].Value != "b" {
			t.Errorf("unexpected content order after replace: %#v", mapping.Content)
		}
	})

	t.Run("append new key", func(t *testing.T) {
		t.Parallel()
		mapping := testMapping("a", "1")
		newVal := yamlx.NewScalarNode("2")

		err := yamlx.Set(mapping, "b", newVal)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mapping.Content) != 4 {
			t.Fatalf("len = %d, want 4", len(mapping.Content))
		}
		got, ok := yamlx.Lookup(mapping, "b")
		if !ok || got != newVal {
			t.Errorf("Lookup(b) = %v, want appended node", got)
		}
	})

	t.Run("set on document node", func(t *testing.T) {
		t.Parallel()
		mapping := testMapping("a", "1")
		doc := newDocumentNode(mapping)
		newVal := yamlx.NewScalarNode("3")

		err := yamlx.Set(doc, "c", newVal)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, ok := yamlx.Lookup(doc, "c")
		if !ok || got != newVal {
			t.Errorf("Lookup(doc, c) = %v, want node", got)
		}
	})

	t.Run("set on empty document node initializes root", func(t *testing.T) {
		t.Parallel()
		doc := &yaml.Node{Kind: yaml.DocumentNode}
		newVal := yamlx.NewScalarNode("first")

		err := yamlx.Set(doc, "key", newVal)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, ok := yamlx.Lookup(doc, "key")
		if !ok || got != newVal {
			t.Errorf("Lookup(doc, key) = %v, want node", got)
		}
	})

	t.Run("set on uninitialized zero node initializes root", func(t *testing.T) {
		t.Parallel()
		var node yaml.Node
		newVal := yamlx.NewScalarNode("first")

		err := yamlx.Set(&node, "key", newVal)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, ok := yamlx.Lookup(&node, "key")
		if !ok || got != newVal {
			t.Errorf("Lookup(node, key) = %v, want node", got)
		}
	})

	t.Run("replace preserves comments when replacement has none", func(t *testing.T) {
		t.Parallel()
		mapping := yamlx.NewMappingNode()
		val := yamlx.NewScalarNode("old")
		val.HeadComment = "# head"
		val.LineComment = "# line"
		val.FootComment = "# foot"
		yamlx.AppendMappingEntry(mapping, "key", val)

		replacement := yamlx.NewScalarNode("new")
		if err := yamlx.Set(mapping, "key", replacement); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, ok := yamlx.Lookup(mapping, "key")
		if !ok {
			t.Fatal("key not found after Set")
		}
		if got.HeadComment != "# head" ||
			got.LineComment != "# line" ||
			got.FootComment != "# foot" {
			t.Errorf("comments not preserved: %#v", got)
		}
	})

	t.Run("replace preserves replacement comments when provided", func(t *testing.T) {
		t.Parallel()
		mapping := yamlx.NewMappingNode()
		val := yamlx.NewScalarNode("old")
		val.HeadComment = "# old head"
		yamlx.AppendMappingEntry(mapping, "key", val)

		replacement := yamlx.NewScalarNode("new")
		replacement.HeadComment = "# new head"
		if err := yamlx.Set(mapping, "key", replacement); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, ok := yamlx.Lookup(mapping, "key")
		if !ok {
			t.Fatal("key not found after Set")
		}
		if got.HeadComment != "# new head" {
			t.Errorf("expected replacement comment preserved: %#v", got)
		}
	})

	t.Run("error on nil mapping", func(t *testing.T) {
		t.Parallel()
		err := yamlx.Set(nil, "key", yamlx.NewScalarNode("val"))
		if err == nil {
			t.Fatal("expected error for nil mapping")
		}
	})

	t.Run("error on non-mapping node", func(t *testing.T) {
		t.Parallel()
		scalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "hello"}
		err := yamlx.Set(scalar, "key", yamlx.NewScalarNode("val"))
		if err == nil {
			t.Fatal("expected error for non-mapping node")
		}
	})

	t.Run("error on nil value node", func(t *testing.T) {
		t.Parallel()
		mapping := yamlx.NewMappingNode()
		err := yamlx.Set(mapping, "key", nil)
		if err == nil {
			t.Fatal("expected error for nil value node")
		}
	})
}

func TestDelete(t *testing.T) {
	t.Parallel()

	mapping := testMapping("a", "1", "b", "2", "c", "3")

	// Delete missing
	if yamlx.Delete(mapping, "nonexistent") {
		t.Error("Delete returned true for nonexistent key")
	}

	// Delete on nil
	if yamlx.Delete(nil, "a") {
		t.Error("Delete returned true for nil mapping")
	}

	// Delete on non-mapping
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "hello"}
	if yamlx.Delete(scalar, "a") {
		t.Error("Delete returned true for scalar node")
	}

	// Delete b
	if !yamlx.Delete(mapping, "b") {
		t.Fatal("Delete failed for existing key b")
	}
	if _, ok := yamlx.Lookup(mapping, "b"); ok {
		t.Error("key b still found after delete")
	}
	if len(mapping.Content) != 4 {
		t.Fatalf("len = %d, want 4", len(mapping.Content))
	}

	// Delete on document node
	doc := newDocumentNode(mapping)
	if !yamlx.Delete(doc, "a") {
		t.Fatal("Delete failed on document node for key a")
	}
	if _, ok := yamlx.Lookup(doc, "a"); ok {
		t.Error("key a still found after delete")
	}

	// Delete remaining c
	if !yamlx.Delete(mapping, "c") {
		t.Fatal("Delete failed for key c")
	}
	if len(mapping.Content) != 0 {
		t.Errorf("len = %d, want 0 after deleting all keys", len(mapping.Content))
	}
}

// testMapping creates a mapping node from alternating key/value strings.
func testMapping(pairs ...string) *yaml.Node {
	if len(pairs)%2 != 0 {
		panic("testMapping requires key/value pairs")
	}

	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, pair := range pairs {
		mapping.Content = append(
			mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: pair},
		)
	}
	return mapping
}

// assertPanics verifies an operation produces any panic value.
func assertPanics(t *testing.T, operation func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("operation did not panic")
		}
	}()
	operation()
}
