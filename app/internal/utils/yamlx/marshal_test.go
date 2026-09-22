package yamlx

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestMarshalPreservesIndentAndComments verifies encoding a parsed node tree
// keeps its comments and applies the requested indentation width.
func TestMarshalPreservesIndentAndComments(t *testing.T) {
	t.Parallel()

	var doc yaml.Node
	if err := yaml.Unmarshal(
		[]byte("root:\n  # keep me\n  child: value\n"),
		&doc,
	); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	out, err := Marshal(&doc, 4)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "# keep me") {
		t.Errorf("Marshal() dropped comment:\n%s", got)
	}
	if !strings.Contains(got, "\n    child: value") {
		t.Errorf("Marshal() did not apply 4-space indent:\n%s", got)
	}
}
