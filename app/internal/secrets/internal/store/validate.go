package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"gopkg.in/yaml.v3"
)

const (
	publicKeysField = "public_keys"
	secretsField    = "secrets"
)

// validate checks the known document fields without discarding unknown fields.
func (d *Document) validate() error {
	root, err := d.topLevel()
	if err != nil {
		return err
	}

	seen := make(map[string]struct{})
	for i := 0; i < len(root.Content); i += 2 {
		if i+1 >= len(root.Content) {
			return errors.New("top-level mapping has an incomplete entry")
		}
		field, err := getStringValue(root.Content[i], "top-level key")
		if err != nil {
			return err
		}
		if _, exists := seen[field]; exists {
			return fmt.Errorf("duplicate top-level field %q", field)
		}
		seen[field] = struct{}{}

		switch field {
		case publicKeysField:
			if err := validatePublicKeys(root.Content[i+1]); err != nil {
				return err
			}
		case secretsField:
			if err := validateSecrets(root.Content[i+1]); err != nil {
				return err
			}
		}
	}
	return nil
}

// validatePublicKeys checks the public_keys mapping and its scalar values.
func validatePublicKeys(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%s must be a mapping", publicKeysField)
	}
	seen := make(map[string]struct{})
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			return fmt.Errorf("%s mapping has an incomplete entry", publicKeysField)
		}
		group, err := getStringValue(node.Content[i], "public key group")
		if err != nil {
			return err
		}
		if err := validateIdentifier("public key group", group); err != nil {
			return err
		}
		normalized := strings.ToLower(group)
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("duplicate public key group %q", group)
		}
		seen[normalized] = struct{}{}

		publicKey, err := getStringValue(node.Content[i+1], "public key")
		if err != nil {
			return err
		}
		if publicKey == "" {
			return fmt.Errorf("public key for group %q is empty", group)
		}
	}
	return nil
}

// validateSecrets checks groups, entry keys, scalar values, and ciphertext
// envelopes while allowing plaintext values for migration workflows.
func validateSecrets(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%s must be a mapping", secretsField)
	}
	seenGroups := make(map[string]struct{})
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			return fmt.Errorf("%s mapping has an incomplete entry", secretsField)
		}
		group, err := getStringValue(node.Content[i], "secret group")
		if err != nil {
			return err
		}
		if err := validateIdentifier("secret group", group); err != nil {
			return err
		}
		normalizedGroup := strings.ToLower(group)
		if _, exists := seenGroups[normalizedGroup]; exists {
			return fmt.Errorf("duplicate secret group %q", group)
		}
		seenGroups[normalizedGroup] = struct{}{}

		groupNode := node.Content[i+1]
		if groupNode.Kind != yaml.MappingNode {
			return fmt.Errorf("secrets group %q is not a mapping", group)
		}
		seenKeys := make(map[string]struct{})
		for j := 0; j < len(groupNode.Content); j += 2 {
			if j+1 >= len(groupNode.Content) {
				return fmt.Errorf("secrets group %q has an incomplete entry", group)
			}
			key, err := getStringValue(groupNode.Content[j], "secret key")
			if err != nil {
				return err
			}
			if err := validateIdentifier("secret key", key); err != nil {
				return err
			}
			if _, exists := seenKeys[key]; exists {
				return fmt.Errorf("duplicate secret key %q in group %q", key, group)
			}
			seenKeys[key] = struct{}{}

			value, err := getStringValue(groupNode.Content[j+1], "secret value")
			if err != nil {
				return err
			}
			if err := validateSecretValue(group, key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateIdentifier rejects empty YAML identifiers used by document methods.
func validateIdentifier(kind, value string) error {
	if value == "" {
		return fmt.Errorf("%s is empty", kind)
	}
	return nil
}

// validateSecretValue accepts plaintext and checks values that claim the
// encrypted envelope format.
func validateSecretValue(group, key, value string) error {
	if !envelope.IsCiphertext(value) {
		return nil
	}
	if err := envelope.Validate(value); err != nil {
		return fmt.Errorf("secret %q in group %q: %w", key, group, err)
	}
	return nil
}
