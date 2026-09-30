package filestore

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/utils/yamlx"
	"gopkg.in/yaml.v3"
)

// validateSecrets checks groups, entry keys, scalar values, and ciphertext
// envelopes while allowing plaintext values for migration workflows.
func (d *document) validateSecrets(node *yaml.Node) error {
	if !yamlx.IsMappingNode(node) {
		return fmt.Errorf("%s must be a mapping", secretsField)
	}
	seenGroups := make(map[string]struct{})
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			return fmt.Errorf("%s mapping has an incomplete entry", secretsField)
		}
		group, err := d.expectString(node.Content[i], "secret group")
		if err != nil {
			return err
		}
		if err := secrets.ValidateSecretGroup(group); err != nil {
			return err
		}
		normalizedGroup := strings.ToLower(group)
		if _, exists := seenGroups[normalizedGroup]; exists {
			return fmt.Errorf("duplicate secret group %q", group)
		}
		seenGroups[normalizedGroup] = struct{}{}

		groupNode := node.Content[i+1]
		if !yamlx.IsMappingNode(groupNode) {
			return fmt.Errorf("secrets group %q is not a mapping", group)
		}
		seenKeys := make(map[string]struct{})
		for j := 0; j < len(groupNode.Content); j += 2 {
			if j+1 >= len(groupNode.Content) {
				return fmt.Errorf("secrets group %q has an incomplete entry", group)
			}
			key, err := d.expectString(groupNode.Content[j], "secret key")
			if err != nil {
				return err
			}
			if err := secrets.ValidateSecretKey(key); err != nil {
				return err
			}
			if _, exists := seenKeys[key]; exists {
				return fmt.Errorf("duplicate secret key %q in group %q", key, group)
			}
			seenKeys[key] = struct{}{}

			value, err := d.expectString(groupNode.Content[j+1], "secret value")
			if err != nil {
				return err
			}
			if err := secrets.ValidateSecretValue(group, key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// secret returns one stored value and whether the entry exists.
func (d *document) secret(group, key string) (secret, bool) {
	if err := secrets.ValidateSecretGroup(group); err != nil {
		return secret{}, false
	}
	if err := secrets.ValidateSecretKey(key); err != nil {
		return secret{}, false
	}

	root, err := d.rootMapping()
	if err != nil {
		return secret{}, false
	}
	secretsEntry, found := yamlx.FindMappingEntry(root, secretsField)
	if !found {
		return secret{}, false
	}
	groupEntry, found := yamlx.FindMappingEntry(
		secretsEntry.ValueNode,
		group,
		yamlx.IgnoreCase,
	)
	if !found {
		return secret{}, false
	}
	keyEntry, found := yamlx.FindMappingEntry(groupEntry.ValueNode, key)
	if !found || !yamlx.IsScalarNode[string](keyEntry.ValueNode) {
		return secret{}, false
	}

	return secret{
		Group: groupEntry.Key,
		Key:   keyEntry.Key,
		Value: keyEntry.ValueNode.Value,
	}, true
}

// secrets returns stored values in group and entry document order.
func (d *document) secrets() []secret {
	result := make([]secret, 0)
	root, err := d.rootMapping()
	if err != nil {
		return result
	}
	secretsEntry, found := yamlx.FindMappingEntry(root, secretsField)
	if !found || !yamlx.IsMappingNode(secretsEntry.ValueNode) {
		return result
	}
	secretsNode := secretsEntry.ValueNode

	for i := 0; i < len(secretsNode.Content); i += 2 {
		if i+1 >= len(secretsNode.Content) {
			return result
		}
		group, err := d.expectString(secretsNode.Content[i], "secret group")
		if err != nil || !yamlx.IsMappingNode(secretsNode.Content[i+1]) {
			continue
		}
		groupNode := secretsNode.Content[i+1]
		for j := 0; j < len(groupNode.Content); j += 2 {
			if j+1 >= len(groupNode.Content) {
				return result
			}
			key, err := d.expectString(groupNode.Content[j], "secret key")
			valueNode := groupNode.Content[j+1]
			if err != nil || !yamlx.IsScalarNode[string](valueNode) {
				continue
			}
			result = append(result, secret{
				Group: group,
				Key:   key,
				Value: valueNode.Value,
			})
		}
	}
	return result
}

// setSecret adds or updates one stored value in memory.
func (d *document) setSecret(group, key, value string) error {
	if err := secrets.ValidateSecretGroup(group); err != nil {
		return err
	}
	if err := secrets.ValidateSecretKey(key); err != nil {
		return err
	}
	if err := secrets.ValidateSecretValue(group, key, value); err != nil {
		return err
	}

	secretsNode, err := d.ensureFieldMapping(secretsField)
	if err != nil {
		return err
	}
	groupEntry, found := yamlx.FindMappingEntry(secretsNode, group, yamlx.IgnoreCase)
	var groupNode *yaml.Node
	if !found {
		groupNode = yamlx.NewMappingNode()
		yamlx.AppendMappingEntry(secretsNode, group, groupNode)
	} else {
		groupNode = groupEntry.ValueNode
	}
	if !yamlx.IsMappingNode(groupNode) {
		return fmt.Errorf("secrets group %q is not a mapping", group)
	}

	entry, found := yamlx.FindMappingEntry(groupNode, key)
	if found {
		if !yamlx.IsScalarNode[string](entry.ValueNode) {
			return fmt.Errorf("secret %q in group %q is not a scalar", key, group)
		}
		yamlx.SetScalarNode(entry.ValueNode, value)
		return nil
	}

	yamlx.AppendMappingEntry(groupNode, key, yamlx.NewScalarNode(value))
	return nil
}

// deleteSecret removes one stored value and reports whether it existed. When the
// removed value was the group's last, the now-empty group mapping is dropped from
// the secrets block, and an emptied secrets block is dropped from the document;
// the group's public key is left untouched so its identity is not torn down.
func (d *document) deleteSecret(group, key string) (bool, error) {
	if err := secrets.ValidateSecretGroup(group); err != nil {
		return false, err
	}
	if err := secrets.ValidateSecretKey(key); err != nil {
		return false, err
	}

	root, err := d.rootMapping()
	if err != nil {
		return false, err
	}
	secretsEntry, found := yamlx.FindMappingEntry(root, secretsField)
	if !found {
		return false, nil
	}
	secretsNode := secretsEntry.ValueNode
	if !yamlx.IsMappingNode(secretsNode) {
		return false, fmt.Errorf("%s must be a mapping", secretsField)
	}

	groupEntry, found := yamlx.FindMappingEntry(secretsNode, group, yamlx.IgnoreCase)
	if !found {
		return false, nil
	}
	groupNode := groupEntry.ValueNode
	if !yamlx.IsMappingNode(groupNode) {
		return false, fmt.Errorf("secrets group %q is not a mapping", group)
	}

	if !yamlx.DeleteMappingEntry(groupNode, key) {
		return false, nil
	}

	// Drop the group mapping once its last value is removed; its public key stays.
	if len(groupNode.Content) == 0 {
		yamlx.DeleteMappingEntry(secretsNode, groupEntry.Key)
	}
	// Drop the whole secrets block once its last group is removed.
	if len(secretsNode.Content) == 0 {
		yamlx.DeleteMappingEntry(root, secretsField)
	}
	return true, nil
}
