package yamlx

import "gopkg.in/yaml.v3"

// IndentLevel infers a document's indentation width from its first nested block
// mapping. It returns false when no block indentation can be detected.
func IndentLevel(document *yaml.Node) (int, bool) {
	if document == nil || len(document.Content) == 0 {
		return 0, false
	}
	if indent, ok := firstNestedIndent(document.Content[0]); ok &&
		indent >= 2 && indent <= 9 {
		return indent, true
	}
	return 0, false
}

// -------------------------------------------------------------------------------------

// firstNestedIndent walks mappings depth-first and returns the column gap
// between a block-mapping key and its first nested block-mapping key.
func firstNestedIndent(node *yaml.Node) (int, bool) {
	if node == nil || node.Kind != yaml.MappingNode || node.Style&yaml.FlowStyle != 0 {
		return 0, false
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		key, value := node.Content[index], node.Content[index+1]
		if value.Kind != yaml.MappingNode || value.Style&yaml.FlowStyle != 0 {
			continue
		}
		if len(value.Content) > 0 {
			if gap := value.Content[0].Column - key.Column; gap > 0 {
				return gap, true
			}
		}
		if gap, ok := firstNestedIndent(value); ok {
			return gap, true
		}
	}
	return 0, false
}
