package filestore

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/lib/strictyaml"
	"github.com/go-envx/envx/app/internal/lib/yamlx"
	"gopkg.in/yaml.v3"
)

const (
	publicKeysField = "public-keys"
	secretsField    = "secrets"

	// suggestionThreshold is the largest edit distance for field suggestions.
	suggestionThreshold = 3
)

// schema is the root shape of a secrets document, used for strict decoding.
type schema struct {
	// PublicKeys maps group names to public keys.
	PublicKeys map[string]string `yaml:"public-keys"`
	// Secrets maps group names to their stored values.
	Secrets map[string]map[string]string `yaml:"secrets"`
}

// document is a validated, editable secrets document that retains comments,
// ordering, and blank lines from its source.
type document struct {
	// root is the editable YAML node tree.
	root yaml.Node
	// source is the original file content, used to restore blank lines.
	source []byte
}

// parseDocument parses and strictly validates secrets YAML. Empty input yields
// an empty document.
func parseDocument(data []byte) (*document, error) {
	doc := &document{source: data}
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &doc.root); err != nil {
			return nil, fmt.Errorf("parsing secrets: %w", err)
		}
	}
	if doc.root.Kind != 0 {
		dec := strictyaml.New(
			strictyaml.WithSchema[schema]("secrets field"),
			strictyaml.WithSuggestionThreshold(suggestionThreshold),
		)
		if err := dec.Decode(data, &schema{}); err != nil {
			return nil, fmt.Errorf("parsing secrets: %w", err)
		}
	}
	if _, err := yamlx.EnsureDocumentMapping(&doc.root); err != nil {
		return nil, fmt.Errorf("parsing secrets: %w", err)
	}
	if err := doc.validate(); err != nil {
		return nil, fmt.Errorf("validating secrets: %w", err)
	}
	return doc, nil
}

// encode validates and serializes the document, preserving its own block
// indentation when it has one.
func (d *document) encode(defaultIndent int) ([]byte, error) {
	if err := d.validate(); err != nil {
		return nil, fmt.Errorf("validating secrets: %w", err)
	}
	indent := defaultIndent
	if own, ok := yamlx.IndentLevel(&d.root); ok {
		indent = own
	}
	data, err := yamlx.Marshal(&d.root, indent)
	if err != nil {
		return nil, fmt.Errorf("encoding secrets: %w", err)
	}
	return yamlx.PreserveBlankLines(d.source, data), nil
}

// rootMapping returns the document's top-level mapping.
func (d *document) rootMapping() (*yaml.Node, error) {
	root, err := yamlx.EnsureDocumentMapping(&d.root)
	if err != nil {
		return nil, errors.New("secrets document must be a mapping")
	}
	return root, nil
}

// field returns a top-level mapping, or nil when it is absent.
func (d *document) field(name string) *yaml.Node {
	root, err := d.rootMapping()
	if err != nil {
		return nil
	}
	entry, found := yamlx.FindMappingEntry(root, name)
	if !found || !yamlx.IsMappingNode(entry.ValueNode) {
		return nil
	}
	return entry.ValueNode
}

// ensureField returns a top-level mapping, appending it when absent.
func (d *document) ensureField(name string) (*yaml.Node, error) {
	if node := d.field(name); node != nil {
		return node, nil
	}
	root, err := d.rootMapping()
	if err != nil {
		return nil, err
	}
	node := yamlx.NewMappingNode()
	yamlx.AppendMappingEntry(root, name, node)
	return node, nil
}

// dropIfEmpty removes an emptied top-level mapping.
func (d *document) dropIfEmpty(name string) {
	node := d.field(name)
	if node == nil || len(node.Content) > 0 {
		return
	}
	if root, err := d.rootMapping(); err == nil {
		yamlx.DeleteMappingEntry(root, name)
	}
}

// groups returns every group in first-seen document order.
func (d *document) groups() []secrets.Group {
	var order []string
	byName := make(map[string]*secrets.Group)
	get := func(name string) *secrets.Group {
		normalized := strings.ToLower(name)
		if group, ok := byName[normalized]; ok {
			return group
		}
		group := &secrets.Group{Name: name, Values: make(map[string]string)}
		byName[normalized] = group
		order = append(order, normalized)
		return group
	}
	if keys := d.field(publicKeysField); keys != nil {
		for i := 0; i+1 < len(keys.Content); i += 2 {
			get(keys.Content[i].Value).PublicKey = keys.Content[i+1].Value
		}
	}
	if values := d.field(secretsField); values != nil {
		for i := 0; i+1 < len(values.Content); i += 2 {
			group := get(values.Content[i].Value)
			entries := values.Content[i+1]
			for j := 0; j+1 < len(entries.Content); j += 2 {
				group.Values[entries.Content[j].Value] = entries.Content[j+1].Value
			}
		}
	}
	out := make([]secrets.Group, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out
}

// replace sets a group's public key and values, editing existing nodes in
// place so comments and ordering survive. New keys are appended in sorted
// order; an emptied group or block is removed.
func (d *document) replace(group secrets.Group) error {
	if err := validateGroup(group); err != nil {
		return err
	}
	if err := d.replacePublicKey(group.Name, group.PublicKey); err != nil {
		return err
	}
	if err := d.replaceValues(group.Name, group.Values); err != nil {
		return err
	}
	return d.validate()
}

// replacePublicKey sets, updates, or removes a group's public key.
func (d *document) replacePublicKey(name, publicKey string) error {
	if publicKey == "" {
		if keys := d.field(publicKeysField); keys != nil {
			yamlx.DeleteMappingEntry(keys, name, yamlx.IgnoreCase)
			d.dropIfEmpty(publicKeysField)
		}
		return nil
	}
	keys, err := d.ensureField(publicKeysField)
	if err != nil {
		return err
	}
	if entry, found := yamlx.FindMappingEntry(keys, name, yamlx.IgnoreCase); found {
		if entry.ValueNode.Value != publicKey {
			yamlx.SetScalarNode(entry.ValueNode, publicKey)
		}
		return nil
	}
	yamlx.AppendMappingEntry(keys, name, yamlx.NewScalarNode(publicKey))
	return nil
}

// replaceValues makes a group's stored values match values exactly.
func (d *document) replaceValues(name string, values map[string]string) error {
	if len(values) == 0 {
		if block := d.field(secretsField); block != nil {
			yamlx.DeleteMappingEntry(block, name, yamlx.IgnoreCase)
			d.dropIfEmpty(secretsField)
		}
		return nil
	}
	block, err := d.ensureField(secretsField)
	if err != nil {
		return err
	}
	var node *yaml.Node
	if entry, found := yamlx.FindMappingEntry(block, name, yamlx.IgnoreCase); found {
		node = entry.ValueNode
	} else {
		node = yamlx.NewMappingNode()
		yamlx.AppendMappingEntry(block, name, node)
	}

	seen := make(map[string]bool, len(values))
	for i := 0; i+1 < len(node.Content); {
		key := node.Content[i].Value
		updated, keep := values[key]
		if !keep {
			yamlx.DeleteMappingEntry(node, key)
			continue
		}
		if node.Content[i+1].Value != updated {
			yamlx.SetScalarNode(node.Content[i+1], updated)
		}
		seen[key] = true
		i += 2
	}
	for _, key := range sortedKeys(values) {
		if !seen[key] {
			yamlx.AppendMappingEntry(node, key, yamlx.NewScalarNode(values[key]))
		}
	}
	return nil
}

// validate checks the known top-level fields and their contents.
func (d *document) validate() error {
	root, err := d.rootMapping()
	if err != nil {
		return err
	}
	seen := make(map[string]bool)
	for i := 0; i < len(root.Content); i += 2 {
		if i+1 >= len(root.Content) {
			return errors.New("top-level mapping has an incomplete entry")
		}
		field, err := expectString(root.Content[i], "top-level key")
		if err != nil {
			return err
		}
		if seen[field] {
			return fmt.Errorf("duplicate top-level field %q", field)
		}
		seen[field] = true
		switch field {
		case publicKeysField:
			err = validatePublicKeys(root.Content[i+1])
		case secretsField:
			err = validateSecrets(root.Content[i+1])
		default:
			err = fmt.Errorf("unknown top-level field %q", field)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// validatePublicKeys checks the public-keys block for non-empty string keys.
func validatePublicKeys(node *yaml.Node) error {
	return eachGroup(node, publicKeysField, func(group string, value *yaml.Node) error {
		publicKey, err := expectString(value, "public key")
		if err != nil {
			return err
		}
		if publicKey == "" {
			return fmt.Errorf("public key for group %q is empty", group)
		}
		return nil
	})
}

// validateSecrets checks the secrets block's keys and stored envelopes.
func validateSecrets(node *yaml.Node) error {
	return eachGroup(node, secretsField, func(group string, entries *yaml.Node) error {
		if !yamlx.IsMappingNode(entries) {
			return fmt.Errorf("secrets group %q is not a mapping", group)
		}
		seen := make(map[string]bool)
		for j := 0; j < len(entries.Content); j += 2 {
			if j+1 >= len(entries.Content) {
				return fmt.Errorf("secrets group %q has an incomplete entry", group)
			}
			key, err := expectString(entries.Content[j], "secret key")
			if err != nil {
				return err
			}
			if err := secrets.ValidateKey(key); err != nil {
				return err
			}
			if seen[key] {
				return fmt.Errorf("duplicate secret key %q in group %q", key, group)
			}
			seen[key] = true
			stored, err := expectString(entries.Content[j+1], "secret value")
			if err != nil {
				return err
			}
			if err := secrets.ValidateEnvelope(stored); err != nil {
				return fmt.Errorf("secret %q in group %q: %w", key, group, err)
			}
		}
		return nil
	})
}

// eachGroup validates a group-keyed mapping's names and uniqueness before
// visiting each value.
func eachGroup(
	node *yaml.Node, field string, visit func(group string, value *yaml.Node) error,
) error {
	if !yamlx.IsMappingNode(node) {
		return fmt.Errorf("%s must be a mapping", field)
	}
	seen := make(map[string]bool)
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			return fmt.Errorf("%s mapping has an incomplete entry", field)
		}
		group, err := expectString(node.Content[i], field+" group")
		if err != nil {
			return err
		}
		normalized, err := secrets.NormalizeGroup(group)
		if err != nil {
			return err
		}
		if seen[normalized] {
			return fmt.Errorf("duplicate %s group %q", field, group)
		}
		seen[normalized] = true
		if err := visit(group, node.Content[i+1]); err != nil {
			return err
		}
	}
	return nil
}

// validateGroup checks a replacement group before it edits the document.
func validateGroup(group secrets.Group) error {
	if _, err := secrets.NormalizeGroup(group.Name); err != nil {
		return err
	}
	for key, stored := range group.Values {
		if err := secrets.ValidateKey(key); err != nil {
			return err
		}
		if err := secrets.ValidateEnvelope(stored); err != nil {
			return fmt.Errorf("secret %q in group %q: %w", key, group.Name, err)
		}
	}
	return nil
}

// expectString returns a string scalar's value.
func expectString(node *yaml.Node, description string) (string, error) {
	if !yamlx.IsScalarNode[string](node) {
		return "", fmt.Errorf("%s must be a string", description)
	}
	return node.Value, nil
}
