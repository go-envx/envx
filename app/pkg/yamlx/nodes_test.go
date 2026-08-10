package yamlx

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestMappingEntry verifies exact and optional case-insensitive lookup while
// returning the key's position in the node content.
func TestMappingEntry(t *testing.T) {
	t.Parallel()

	mapping := testMapping("First", "one", "Production", "two")
	tests := []struct {
		name       string
		key        string
		ignoreCase bool
		wantValue  string
		wantIndex  int
	}{
		{
			name:      "exact first entry",
			key:       "First",
			wantValue: "one",
			wantIndex: 0,
		},
		{
			name:      "case-sensitive mismatch",
			key:       "production",
			wantIndex: -1,
		},
		{
			name:       "case-insensitive second entry",
			key:        "production",
			ignoreCase: true,
			wantValue:  "two",
			wantIndex:  2,
		},
		{
			name:      "missing",
			key:       "missing",
			wantIndex: -1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, index := MappingEntry(mapping, test.key, test.ignoreCase)
			if index != test.wantIndex {
				t.Fatalf("MappingEntry() index = %d, want %d", index, test.wantIndex)
			}
			if test.wantIndex < 0 {
				if got != nil {
					t.Fatalf("MappingEntry() node = %#v, want nil", got)
				}
				return
			}
			if got == nil || got.Value != test.wantValue {
				t.Errorf("MappingEntry() value = %#v, want %q", got, test.wantValue)
			}
		})
	}
}

// TestMappingEntryReturnsNotFoundForInvalidNodes verifies nil and non-mapping
// nodes are treated as unsuccessful lookups.
func TestMappingEntryReturnsNotFoundForInvalidNodes(t *testing.T) {
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
			got, index := MappingEntry(test.node, "key", false)
			if got != nil || index != -1 {
				t.Errorf("MappingEntry() = (%#v, %d), want (nil, -1)", got, index)
			}
		})
	}
}

// TestRemoveMappingEntry verifies pair removal preserves the remaining order.
func TestRemoveMappingEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		index     int
		wantPairs []string
	}{
		{
			name:      "first",
			index:     0,
			wantPairs: []string{"Second", "two", "Third", "three"},
		},
		{
			name:      "middle",
			index:     2,
			wantPairs: []string{"First", "one", "Third", "three"},
		},
		{
			name:      "last",
			index:     4,
			wantPairs: []string{"First", "one", "Second", "two"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapping := testMapping(
				"First", "one", "Second", "two", "Third", "three",
			)
			RemoveMappingEntry(mapping, test.index)

			if len(mapping.Content) != len(test.wantPairs) {
				t.Fatalf(
					"RemoveMappingEntry() content length = %d, want %d",
					len(mapping.Content), len(test.wantPairs),
				)
			}
			for index, want := range test.wantPairs {
				if got := mapping.Content[index].Value; got != want {
					t.Errorf(
						"RemoveMappingEntry() content[%d] = %q, want %q",
						index, got, want,
					)
				}
			}
		})
	}
}

// TestRemoveMappingEntryPanicsForInvalidEntries verifies malformed mappings and
// non-entry indices fail before mutating the node.
func TestRemoveMappingEntryPanicsForInvalidEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mapping *yaml.Node
		index   int
	}{
		{name: "nil mapping", index: 0},
		{
			name:    "non-mapping",
			mapping: &yaml.Node{Kind: yaml.ScalarNode},
			index:   0,
		},
		{
			name:    "incomplete mapping",
			mapping: &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{}}},
			index:   0,
		},
		{name: "negative index", mapping: testMapping("key", "value"), index: -1},
		{name: "out of range", mapping: testMapping("key", "value"), index: 2},
		{name: "value index", mapping: testMapping("key", "value"), index: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertPanics(t, func() {
				RemoveMappingEntry(test.mapping, test.index)
			})
		})
	}
}

// TestSetStringScalar verifies rewriting a node clears YAML value state while
// preserving comments and source position metadata.
func TestSetStringScalar(t *testing.T) {
	t.Parallel()

	child := &yaml.Node{Kind: yaml.ScalarNode, Value: "child"}
	node := &yaml.Node{
		Kind:        yaml.MappingNode,
		Tag:         "!!map",
		Style:       yaml.FlowStyle,
		Value:       "old",
		Content:     []*yaml.Node{child},
		HeadComment: "head",
		LineComment: "line",
		FootComment: "foot",
		Line:        4,
		Column:      7,
	}

	SetStringScalar(node, "new: value")

	if node.Kind != yaml.ScalarNode {
		t.Errorf("SetStringScalar() kind = %v, want scalar", node.Kind)
	}
	if node.Tag != "!!str" {
		t.Errorf("SetStringScalar() tag = %q, want !!str", node.Tag)
	}
	if node.Style != 0 {
		t.Errorf("SetStringScalar() style = %v, want 0", node.Style)
	}
	if node.Value != "new: value" {
		t.Errorf("SetStringScalar() value = %q, want %q", node.Value, "new: value")
	}
	if node.Content != nil {
		t.Errorf("SetStringScalar() content = %#v, want nil", node.Content)
	}
	if node.HeadComment != "head" ||
		node.LineComment != "line" ||
		node.FootComment != "foot" ||
		node.Line != 4 ||
		node.Column != 7 {
		t.Errorf("SetStringScalar() changed node metadata: %#v", node)
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
		if recover() == nil {
			t.Error("operation did not panic")
		}
	}()
	operation()
}
