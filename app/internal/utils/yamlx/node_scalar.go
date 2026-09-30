package yamlx

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// ScalarValue constrains types that represent YAML scalar values.
type ScalarValue interface {
	~string | ~bool |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// IsScalarNode reports whether node is a non-nil scalar node with a tag
// matching the expected YAML tag for T.
func IsScalarNode[T ScalarValue](node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.ScalarNode {
		return false
	}
	var zero T
	tag, _ := formatScalar(zero)
	return node.Tag == tag
}

// NewScalarNode creates a new scalar node with an auto-derived tag.
func NewScalarNode[T ScalarValue](value T) *yaml.Node {
	node := &yaml.Node{}
	SetScalarNode(node, value)
	return node
}

// SetScalarNode formats any scalar primitive into node, auto-deriving the YAML tag
// (!!str, !!int, !!bool, !!float) without runtime errors, while preserving
// existing comments.
func SetScalarNode[T ScalarValue](scalar *yaml.Node, value T) {
	tag, strVal := formatScalar(value)
	scalar.Kind = yaml.ScalarNode
	scalar.Tag = tag
	scalar.Style = 0
	scalar.Value = strVal
	scalar.Content = nil
}

// formatScalar converts a supported scalar primitive into its canonical YAML
// representation and default YAML tag (!!str, !!bool, !!int, !!float).
func formatScalar[T ScalarValue](value T) (tag, strVal string) {
	var val any = value
	switch v := val.(type) {
	case string:
		return "!!str", v
	case bool:
		if v {
			return "!!bool", "true"
		}
		return "!!bool", "false"
	case int:
		return "!!int", strconv.FormatInt(int64(v), 10)
	case int8:
		return "!!int", strconv.FormatInt(int64(v), 10)
	case int16:
		return "!!int", strconv.FormatInt(int64(v), 10)
	case int32:
		return "!!int", strconv.FormatInt(int64(v), 10)
	case int64:
		return "!!int", strconv.FormatInt(v, 10)
	case uint:
		return "!!int", strconv.FormatUint(uint64(v), 10)
	case uint8:
		return "!!int", strconv.FormatUint(uint64(v), 10)
	case uint16:
		return "!!int", strconv.FormatUint(uint64(v), 10)
	case uint32:
		return "!!int", strconv.FormatUint(uint64(v), 10)
	case uint64:
		return "!!int", strconv.FormatUint(v, 10)
	case float32:
		return "!!float", strconv.FormatFloat(float64(v), 'g', -1, 32)
	case float64:
		return "!!float", strconv.FormatFloat(v, 'g', -1, 64)
	default:
		return "!!str", fmt.Sprint(v)
	}
}
