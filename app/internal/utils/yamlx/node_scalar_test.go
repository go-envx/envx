package yamlx_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/utils/yamlx"
	"gopkg.in/yaml.v3"
)

func TestNewScalarNode(t *testing.T) {
	t.Parallel()

	node := yamlx.NewScalarNode(123)
	if node == nil {
		t.Fatal("NewScalarNode returned nil")
	}
	if node.Kind != yaml.ScalarNode || node.Tag != "!!int" || node.Value != "123" {
		t.Errorf("NewScalarNode(123) = %#v", node)
	}
}

func TestSetScalarNode(t *testing.T) {
	t.Parallel()

	t.Run("preserves comments and metadata", func(t *testing.T) {
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

		yamlx.SetScalarNode(node, "new: value")

		if node.Kind != yaml.ScalarNode {
			t.Errorf("SetScalarNode() kind = %v, want scalar", node.Kind)
		}
		if node.Tag != "!!str" {
			t.Errorf("SetScalarNode() tag = %q, want !!str", node.Tag)
		}
		if node.Style != 0 {
			t.Errorf("SetScalarNode() style = %v, want 0", node.Style)
		}
		if node.Value != "new: value" {
			t.Errorf("SetScalarNode() value = %q, want %q", node.Value, "new: value")
		}
		if node.Content != nil {
			t.Errorf("SetScalarNode() content = %#v, want nil", node.Content)
		}
		if node.HeadComment != "head" ||
			node.LineComment != "line" ||
			node.FootComment != "foot" ||
			node.Line != 4 ||
			node.Column != 7 {
			t.Errorf("SetScalarNode() changed node metadata: %#v", node)
		}
	})

	t.Run("formats various scalar types with correct tags", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			apply     func(node *yaml.Node)
			wantTag   string
			wantValue string
		}{
			{
				name:      "string",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, "hello") },
				wantTag:   "!!str",
				wantValue: "hello",
			},
			{
				name:      "bool true",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, true) },
				wantTag:   "!!bool",
				wantValue: "true",
			},
			{
				name:      "bool false",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, false) },
				wantTag:   "!!bool",
				wantValue: "false",
			},
			{
				name:      "int",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, 42) },
				wantTag:   "!!int",
				wantValue: "42",
			},
			{
				name:      "int8",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, int8(-8)) },
				wantTag:   "!!int",
				wantValue: "-8",
			},
			{
				name:      "int16",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, int16(16)) },
				wantTag:   "!!int",
				wantValue: "16",
			},
			{
				name:      "int32",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, int32(-32)) },
				wantTag:   "!!int",
				wantValue: "-32",
			},
			{
				name:      "int64",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, int64(1234567890)) },
				wantTag:   "!!int",
				wantValue: "1234567890",
			},
			{
				name:      "uint",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, uint(10)) },
				wantTag:   "!!int",
				wantValue: "10",
			},
			{
				name:      "uint8",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, uint8(255)) },
				wantTag:   "!!int",
				wantValue: "255",
			},
			{
				name:      "uint16",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, uint16(65535)) },
				wantTag:   "!!int",
				wantValue: "65535",
			},
			{
				name:      "uint32",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, uint32(4000000000)) },
				wantTag:   "!!int",
				wantValue: "4000000000",
			},
			{
				name: "uint64",
				apply: func(n *yaml.Node) {
					yamlx.SetScalarNode(n, uint64(9000000000000000000))
				},
				wantTag:   "!!int",
				wantValue: "9000000000000000000",
			},
			{
				name:      "float32",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, float32(3.14)) },
				wantTag:   "!!float",
				wantValue: "3.14",
			},
			{
				name:      "float64",
				apply:     func(n *yaml.Node) { yamlx.SetScalarNode(n, 2.71828) },
				wantTag:   "!!float",
				wantValue: "2.71828",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				node := &yaml.Node{}
				test.apply(node)
				if node.Kind != yaml.ScalarNode {
					t.Errorf("Kind = %v, want ScalarNode", node.Kind)
				}
				if node.Tag != test.wantTag {
					t.Errorf("Tag = %q, want %q", node.Tag, test.wantTag)
				}
				if node.Value != test.wantValue {
					t.Errorf("Value = %q, want %q", node.Value, test.wantValue)
				}
			})
		}
	})
}

func TestIsScalarNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		node *yaml.Node
		test func(t *testing.T, n *yaml.Node)
	}{
		{
			name: "nil node",
			node: nil,
			test: func(t *testing.T, n *yaml.Node) {
				t.Helper()
				if yamlx.IsScalarNode[string](n) {
					t.Error("IsScalarNode[string](nil) = true, want false")
				}
				if yamlx.IsScalarNode[int](n) {
					t.Error("IsScalarNode[int](nil) = true, want false")
				}
			},
		},
		{
			name: "string scalar",
			node: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "hello"},
			test: func(t *testing.T, n *yaml.Node) {
				t.Helper()
				if !yamlx.IsScalarNode[string](n) {
					t.Error("IsScalarNode[string] = false, want true")
				}
				if yamlx.IsScalarNode[int](n) {
					t.Error("IsScalarNode[int] = true, want false")
				}
			},
		},
		{
			name: "int scalar",
			node: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "123"},
			test: func(t *testing.T, n *yaml.Node) {
				t.Helper()
				if !yamlx.IsScalarNode[int](n) {
					t.Error("IsScalarNode[int] = false, want true")
				}
				if yamlx.IsScalarNode[string](n) {
					t.Error("IsScalarNode[string] = true, want false")
				}
			},
		},
		{
			name: "bool scalar",
			node: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
			test: func(t *testing.T, n *yaml.Node) {
				t.Helper()
				if !yamlx.IsScalarNode[bool](n) {
					t.Error("IsScalarNode[bool] = false, want true")
				}
				if yamlx.IsScalarNode[string](n) {
					t.Error("IsScalarNode[string] = true, want false")
				}
			},
		},
		{
			name: "float scalar",
			node: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: "3.14"},
			test: func(t *testing.T, n *yaml.Node) {
				t.Helper()
				if !yamlx.IsScalarNode[float64](n) {
					t.Error("IsScalarNode[float64] = false, want true")
				}
				if yamlx.IsScalarNode[string](n) {
					t.Error("IsScalarNode[string] = true, want false")
				}
			},
		},
		{
			name: "mapping node",
			node: &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"},
			test: func(t *testing.T, n *yaml.Node) {
				t.Helper()
				if yamlx.IsScalarNode[string](n) {
					t.Error("IsScalarNode[string] on map = true, want false")
				}
			},
		},
		{
			name: "sequence node",
			node: &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"},
			test: func(t *testing.T, n *yaml.Node) {
				t.Helper()
				if yamlx.IsScalarNode[string](n) {
					t.Error("IsScalarNode[string] on seq = true, want false")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.test(t, tc.node)
		})
	}
}
