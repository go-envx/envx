package store

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// -------------------------------------------------------------------------------------

// SetPublicKey adds or updates a group's public key in memory.
func (d *Document) SetPublicKey(group, publicKey string) error {
	if err := validateIdentifier("secret group", group); err != nil {
		return err
	}
	if publicKey == "" {
		return errors.New("public key is empty")
	}

	publicKeys, err := d.ensureFieldMapping(publicKeysField)
	if err != nil {
		return err
	}
	entry, err := getMappingEntry(publicKeys, group, true)
	if err != nil {
		return err
	}
	if entry.found {
		if entry.value.Kind != yaml.ScalarNode {
			return fmt.Errorf("public key for group %q is not a scalar", group)
		}
		entry.value.Tag = "!!str"
		entry.value.Value = publicKey
		return nil
	}

	appendMappingEntry(publicKeys, group, publicKey)
	return nil
}

// -------------------------------------------------------------------------------------

// SetSecret adds or updates one stored value in memory.
func (d *Document) SetSecret(group, key, value string) error {
	if err := validateIdentifier("secret group", group); err != nil {
		return err
	}
	if err := validateIdentifier("secret key", key); err != nil {
		return err
	}
	if err := validateSecretValue(group, key, value); err != nil {
		return err
	}

	secrets, err := d.ensureFieldMapping(secretsField)
	if err != nil {
		return err
	}
	groupEntry, err := getMappingEntry(secrets, group, true)
	if err != nil {
		return err
	}
	groupNode := groupEntry.value
	if !groupEntry.found {
		groupNode = newMappingNode()
		appendMappingNode(secrets, group, groupNode)
	}
	if groupNode.Kind != yaml.MappingNode {
		return fmt.Errorf("secrets group %q is not a mapping", group)
	}

	entry, err := getMappingEntry(groupNode, key, false)
	if err != nil {
		return err
	}
	if entry.found {
		if entry.value.Kind != yaml.ScalarNode {
			return fmt.Errorf("secret %q in group %q is not a scalar", key, group)
		}
		entry.value.Tag = "!!str"
		entry.value.Value = value
		return nil
	}

	appendMappingEntry(groupNode, key, value)
	return nil
}

// -------------------------------------------------------------------------------------

// DeleteSecret removes one stored value and reports whether it existed. The
// containing group remains in the document when its last value is removed.
func (d *Document) DeleteSecret(group, key string) (bool, error) {
	if err := validateIdentifier("secret group", group); err != nil {
		return false, err
	}
	if err := validateIdentifier("secret key", key); err != nil {
		return false, err
	}

	root, err := d.topLevel()
	if err != nil {
		return false, err
	}
	secretsEntry, err := getMappingEntry(root, secretsField, false)
	if err != nil || !secretsEntry.found {
		return false, err
	}
	secrets := secretsEntry.value
	if secrets.Kind != yaml.MappingNode {
		return false, fmt.Errorf("%s must be a mapping", secretsField)
	}

	groupEntry, err := getMappingEntry(secrets, group, true)
	if err != nil || !groupEntry.found {
		return false, err
	}
	groupNode := groupEntry.value
	if groupNode.Kind != yaml.MappingNode {
		return false, fmt.Errorf("secrets group %q is not a mapping", group)
	}

	keyEntry, err := getMappingEntry(groupNode, key, false)
	if err != nil || !keyEntry.found {
		return false, err
	}
	removeMappingEntry(groupNode, keyEntry.index)
	return true, nil
}

// -------------------------------------------------------------------------------------

// ensureFieldMapping returns a known top-level mapping, creating it when absent.
func (d *Document) ensureFieldMapping(field string) (*yaml.Node, error) {
	root, err := d.topLevel()
	if err != nil {
		return nil, err
	}
	entry, err := getMappingEntry(root, field, false)
	if err != nil {
		return nil, err
	}
	if entry.found {
		if entry.value.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s must be a mapping", field)
		}
		return entry.value, nil
	}

	node := newMappingNode()
	appendMappingNode(root, field, node)
	return node, nil
}
