package yamlx

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// -------------------------------------------------------------------------------------

// TestIndentLevel verifies indentation is inferred from nested block mappings.
func TestIndentLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		body  string
		want  int
		found bool
	}{
		{
			name:  "two spaces",
			body:  "root:\n  child:\n    value: one\n",
			want:  2,
			found: true,
		},
		{
			name:  "four spaces",
			body:  "root:\n    child:\n        value: one\n",
			want:  4,
			found: true,
		},
		{
			name:  "flat",
			body:  "root: value\n",
			found: false,
		},
		{
			name:  "flow mapping",
			body:  "root: {child: value}\n",
			found: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := new(yaml.Node)
			if err := yaml.Unmarshal([]byte(test.body), document); err != nil {
				t.Fatalf("yaml.Unmarshal() error = %v", err)
			}
			got, found := IndentLevel(document)
			if got != test.want || found != test.found {
				t.Errorf(
					"IndentLevel() = (%d, %t), want (%d, %t)",
					got, found, test.want, test.found,
				)
			}
		})
	}
}

// -------------------------------------------------------------------------------------

// TestIndentLevelReturnsNoMatchForEmptyAndNil verifies missing node content is
// reported as undetectable rather than assigned a package-level default.
func TestIndentLevelReturnsNoMatchForEmptyAndNil(t *testing.T) {
	t.Parallel()

	if got, found := IndentLevel(nil); got != 0 || found {
		t.Errorf("IndentLevel(nil) = (%d, %t), want (0, false)", got, found)
	}
	if got, found := IndentLevel(new(yaml.Node)); got != 0 || found {
		t.Errorf("IndentLevel(empty) = (%d, %t), want (0, false)", got, found)
	}
}
