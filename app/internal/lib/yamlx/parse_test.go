package yamlx_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/yamlx"
	"gopkg.in/yaml.v3"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		yamlData  string
		wantErr   bool
		errTarget error
		validate  func(t *testing.T, node *yaml.Node)
	}{
		{
			name:     "empty input",
			yamlData: "",
			validate: func(t *testing.T, node *yaml.Node) {
				if node == nil || node.Kind != yaml.DocumentNode {
					t.Fatalf("expected DocumentNode, got %v", node)
				}
			},
		},
		{
			name:     "comment only",
			yamlData: "# just a comment\n",
			validate: func(t *testing.T, node *yaml.Node) {
				if node == nil || node.Kind != yaml.DocumentNode {
					t.Fatalf("expected DocumentNode, got %v", node)
				}
			},
		},
		{
			name:     "valid single mapping document",
			yamlData: "key: value\nnumber: 42\n",
			validate: func(t *testing.T, node *yaml.Node) {
				if node == nil || node.Kind != yaml.DocumentNode {
					t.Fatalf("expected DocumentNode, got %v", node)
				}
				if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
					t.Fatalf("expected mapping root, got %v", node.Content)
				}
			},
		},
		{
			name:      "trailing document rejected",
			yamlData:  "doc1: true\n---\ndoc2: false\n",
			wantErr:   true,
			errTarget: yamlx.ErrTrailingDocument,
		},
		{
			name:      "duplicate key at top level rejected",
			yamlData:  "key: first\nkey: second\n",
			wantErr:   true,
			errTarget: yamlx.ErrDuplicateKey,
		},
		{
			name: "duplicate key nested mapping rejected",
			yamlData: `
parent:
  child1: val1
  nested:
    dup: 1
    dup: 2
`,
			wantErr:   true,
			errTarget: yamlx.ErrDuplicateKey,
		},
		{
			name: "duplicate key in sequence mapping rejected",
			yamlData: `
items:
  - id: 1
    id: 2
`,
			wantErr:   true,
			errTarget: yamlx.ErrDuplicateKey,
		},
		{
			name:     "syntax error",
			yamlData: "key: [unclosed\n",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			node, err := yamlx.Parse([]byte(tc.yamlData))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errTarget != nil && !errors.Is(err, tc.errTarget) {
					t.Fatalf("expected error wrapping %v, got %v", tc.errTarget, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.validate != nil {
				tc.validate(t, node)
			}
		})
	}
}
