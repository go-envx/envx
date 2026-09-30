package filestore

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/utils/yamlx"
	"gopkg.in/yaml.v3"
)

// validatePublicKeys checks the public-keys mapping and its scalar values.
func (d *document) validatePublicKeys(node *yaml.Node) error {
	if !yamlx.IsMappingNode(node) {
		return fmt.Errorf("%s must be a mapping", publicKeysField)
	}
	seen := make(map[string]struct{})
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			return fmt.Errorf("%s mapping has an incomplete entry", publicKeysField)
		}
		group, err := d.expectString(node.Content[i], "public key group")
		if err != nil {
			return err
		}
		if err := secrets.ValidateSecretGroup(group); err != nil {
			return err
		}
		normalized := strings.ToLower(group)
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("duplicate public key group %q", group)
		}
		seen[normalized] = struct{}{}

		publicKey, err := d.expectString(node.Content[i+1], "public key")
		if err != nil {
			return err
		}
		if publicKey == "" {
			return fmt.Errorf("public key for group %q is empty", group)
		}
	}
	return nil
}

// publicKey returns a group's stored public key and whether the group exists.
func (d *document) publicKey(group string) (string, bool) {
	if err := secrets.ValidateSecretGroup(group); err != nil {
		return "", false
	}

	root, err := d.rootMapping()
	if err != nil {
		return "", false
	}
	publicKeysEntry, found := yamlx.FindMappingEntry(root, publicKeysField)
	if !found {
		return "", false
	}
	publicKeyEntry, found := yamlx.FindMappingEntry(
		publicKeysEntry.ValueNode,
		group,
		yamlx.IgnoreCase,
	)
	if !found {
		return "", false
	}
	value, err := d.expectString(publicKeyEntry.ValueNode, "public key")
	if err != nil {
		return "", false
	}
	return value, true
}

// publicKeyGroups returns the names of every group that declares a public key,
// in document order. It reads the public-keys block without exposing any key
// material.
func (d *document) publicKeyGroups() []string {
	root, err := d.rootMapping()
	if err != nil {
		return []string{}
	}
	entry, found := yamlx.FindMappingEntry(root, publicKeysField)
	if !found || !yamlx.IsMappingNode(entry.ValueNode) {
		return []string{}
	}
	groups := entry.ValueNode
	result := make([]string, 0, len(groups.Content)/2)
	for i := 0; i+1 < len(groups.Content); i += 2 {
		name, err := d.expectString(groups.Content[i], "public key group")
		if err != nil {
			continue
		}
		result = append(result, name)
	}
	return result
}

// setPublicKey adds or updates a group's public key in memory.
func (d *document) setPublicKey(group, publicKey string) error {
	if err := secrets.ValidateSecretGroup(group); err != nil {
		return err
	}
	if publicKey == "" {
		return errors.New("public key is empty")
	}

	publicKeys, err := d.ensureFieldMapping(publicKeysField)
	if err != nil {
		return err
	}
	entry, found := yamlx.FindMappingEntry(publicKeys, group, yamlx.IgnoreCase)
	if found {
		if !yamlx.IsScalarNode[string](entry.ValueNode) {
			return fmt.Errorf("public key for group %q is not a scalar", group)
		}
		yamlx.SetScalarNode(entry.ValueNode, publicKey)
		return nil
	}

	yamlx.AppendMappingEntry(publicKeys, group, yamlx.NewScalarNode(publicKey))
	return nil
}
