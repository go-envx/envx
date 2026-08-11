package yamlx

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

// Marshal encodes node as YAML using indent spaces per nesting level. Encoding a
// pre-parsed node tree preserves its comments, key order, and styles, so callers
// can rewrite a document without reflowing untouched parts.
func Marshal(node *yaml.Node, indent int) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(indent)
	if err := encoder.Encode(node); err != nil {
		_ = encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
