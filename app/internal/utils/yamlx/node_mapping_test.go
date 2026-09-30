package yamlx_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/utils/yamlx"
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
				if !entry.IsEmpty() {
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
			if found || !entry.IsEmpty() || entry.Index != -1 {
				t.Errorf("FindMappingEntry() = (%#v, %v), want empty", entry, found)
			}
		})
	}
}

func TestSetMappingEntry(t *testing.T) {
	t.Parallel()

	t.Run("appends when absent", func(t *testing.T) {
		t.Parallel()
		mapping := yamlx.NewMappingNode()
		val := yamlx.NewScalarNode("value")
		yamlx.SetMappingEntry(mapping, "key", val)

		entry, found := yamlx.FindMappingEntry(mapping, "key")
		if !found {
			t.Fatal("key not found after SetMappingEntry")
		}
		if entry.ValueNode != val {
			t.Errorf("entry.ValueNode = %#v, want %#v", entry.ValueNode, val)
		}
	})

	t.Run("updates value and keeps key comment when present", func(t *testing.T) {
		t.Parallel()
		mapping := yamlx.NewMappingNode()
		keyNode := &yaml.Node{
			Kind:        yaml.ScalarNode,
			Tag:         "!!str",
			Value:       "myKey",
			HeadComment: "# Key comment",
			LineComment: "# inline comment",
		}
		valNode := &yaml.Node{
			Kind:        yaml.ScalarNode,
			Tag:         "!!str",
			Value:       "oldValue",
			HeadComment: "# Val head",
			LineComment: "# Val inline",
			FootComment: "# Val foot",
		}
		mapping.Content = []*yaml.Node{keyNode, valNode}

		newVal := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: "newValue",
		}

		yamlx.SetMappingEntry(mapping, "mykey", newVal, yamlx.IgnoreCase)

		entry, found := yamlx.FindMappingEntry(mapping, "mykey", yamlx.IgnoreCase)
		if !found {
			t.Fatal("key not found")
		}
		// Preserved key node and comments
		if entry.KeyNode != keyNode {
			t.Errorf("keyNode changed")
		}
		if entry.KeyNode.HeadComment != "# Key comment" ||
			entry.KeyNode.LineComment != "# inline comment" {
			t.Errorf("key comments changed: %#v", entry.KeyNode)
		}
		// Value replaced with comments copied from old value
		if entry.ValueNode.Value != "newValue" {
			t.Errorf("value = %q, want newValue", entry.ValueNode.Value)
		}
		if entry.ValueNode.HeadComment != "# Val head" ||
			entry.ValueNode.LineComment != "# Val inline" ||
			entry.ValueNode.FootComment != "# Val foot" {
			t.Errorf("value comments not copied over: %#v", entry.ValueNode)
		}
	})

	t.Run("does not overwrite new value comments if provided", func(t *testing.T) {
		t.Parallel()
		mapping := yamlx.NewMappingNode()
		oldVal := &yaml.Node{
			Kind:        yaml.ScalarNode,
			Tag:         "!!str",
			Value:       "oldValue",
			HeadComment: "# Old head",
		}
		mapping.Content = []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "key"},
			oldVal,
		}

		newVal := &yaml.Node{
			Kind:        yaml.ScalarNode,
			Tag:         "!!str",
			Value:       "newValue",
			HeadComment: "# New head",
		}

		yamlx.SetMappingEntry(mapping, "key", newVal)

		entry, found := yamlx.FindMappingEntry(mapping, "key")
		if !found {
			t.Fatal("key not found")
		}
		if entry.ValueNode.HeadComment != "# New head" {
			t.Errorf("headComment = %q, want '# New head'", entry.ValueNode.HeadComment)
		}
	})

	t.Run("panics on invalid node", func(t *testing.T) {
		t.Parallel()
		assertPanics(t, func() {
			yamlx.SetMappingEntry(nil, "key", yamlx.NewScalarNode("val"))
		})
		assertPanics(t, func() {
			yamlx.SetMappingEntry(
				&yaml.Node{Kind: yaml.ScalarNode},
				"key",
				yamlx.NewScalarNode("val"),
			)
		})
	})
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
