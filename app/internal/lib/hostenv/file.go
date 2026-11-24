package hostenv

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// syntaxError is a parse failure on one line.
type syntaxError struct {
	line int
	msg  string
}

func (e *syntaxError) Error() string {
	return fmt.Sprintf("line %d: %s", e.line, e.msg)
}

// parseFile parses content as YAML for .yaml and .yml files, and as dotenv
// otherwise.
func parseFile(path string, data []byte) (map[string]string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return parseYAML(data)
	default:
		return godotenv.Unmarshal(string(data))
	}
}

// parseYAML parses a flat mapping of variable names to string values. Nested
// values, non-string scalars, aliases, and duplicate keys are errors.
func parseYAML(data []byte) (map[string]string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	values := make(map[string]string)
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return values, nil
	}
	root := doc.Content[0]
	if root.Kind == yaml.ScalarNode && root.Tag == "!!null" {
		return values, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, &syntaxError{root.Line, "expected a mapping of NAME: value"}
	}
	for i := 0; i < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		if key.Kind != yaml.ScalarNode || !validName(key.Value) {
			return nil, &syntaxError{
				key.Line, fmt.Sprintf("invalid variable name %q", key.Value),
			}
		}
		if _, dup := values[key.Value]; dup {
			return nil, &syntaxError{
				key.Line, fmt.Sprintf("duplicate variable %q", key.Value),
			}
		}
		if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
			return nil, &syntaxError{
				value.Line,
				fmt.Sprintf("value of %s must be a string; quote it", key.Value),
			}
		}
		values[key.Value] = value.Value
	}
	return values, nil
}

// validName reports whether name matches [A-Za-z_][A-Za-z0-9_]*.
func validName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		letter := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// fileError names path in a parse failure, with its line when known.
func fileError(path string, err error) error {
	if syntax, ok := errors.AsType[*syntaxError](err); ok {
		return fmt.Errorf("%s:%d: %s", path, syntax.line, syntax.msg)
	}
	return fmt.Errorf("%s: %w", path, err)
}
