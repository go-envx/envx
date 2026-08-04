package store

import "gopkg.in/yaml.v3"

// -------------------------------------------------------------------------------------

// PublicKey returns a group's stored public key and whether the group exists.
func (d *Document) PublicKey(group string) (string, bool) {
	if group == "" {
		return "", false
	}

	root, err := d.topLevel()
	if err != nil {
		return "", false
	}
	publicKeysEntry, err := getMappingEntry(root, publicKeysField, false)
	if err != nil || !publicKeysEntry.found {
		return "", false
	}
	publicKeyEntry, err := getMappingEntry(publicKeysEntry.value, group, true)
	if err != nil || !publicKeyEntry.found {
		return "", false
	}
	value, err := getStringValue(publicKeyEntry.value, "public key")
	if err != nil {
		return "", false
	}
	return value, true
}

// -------------------------------------------------------------------------------------

// Secret returns one stored value and whether the entry exists.
func (d *Document) Secret(group, key string) (Secret, bool) {
	if group == "" || key == "" {
		return Secret{}, false
	}

	root, err := d.topLevel()
	if err != nil {
		return Secret{}, false
	}
	secretsEntry, err := getMappingEntry(root, secretsField, false)
	if err != nil || !secretsEntry.found {
		return Secret{}, false
	}
	groupEntry, err := getMappingEntry(secretsEntry.value, group, true)
	if err != nil || !groupEntry.found {
		return Secret{}, false
	}
	keyEntry, err := getMappingEntry(groupEntry.value, key, false)
	if err != nil || !keyEntry.found || keyEntry.value.Kind != yaml.ScalarNode {
		return Secret{}, false
	}

	return Secret{
		Group: groupEntry.key,
		Key:   keyEntry.key,
		Value: keyEntry.value.Value,
	}, true
}

// -------------------------------------------------------------------------------------

// Secrets returns stored values in group and entry document order.
func (d *Document) Secrets() []Secret {
	result := make([]Secret, 0)
	root, err := d.topLevel()
	if err != nil {
		return result
	}
	secretsEntry, err := getMappingEntry(root, secretsField, false)
	if err != nil || !secretsEntry.found || secretsEntry.value.Kind != yaml.MappingNode {
		return result
	}
	secrets := secretsEntry.value

	for i := 0; i < len(secrets.Content); i += 2 {
		if i+1 >= len(secrets.Content) {
			return result
		}
		group, err := getStringValue(secrets.Content[i], "secret group")
		if err != nil || secrets.Content[i+1].Kind != yaml.MappingNode {
			continue
		}
		groupNode := secrets.Content[i+1]
		for j := 0; j < len(groupNode.Content); j += 2 {
			if j+1 >= len(groupNode.Content) {
				return result
			}
			key, err := getStringValue(groupNode.Content[j], "secret key")
			valueNode := groupNode.Content[j+1]
			if err != nil || valueNode.Kind != yaml.ScalarNode {
				continue
			}
			result = append(result, Secret{
				Group: group,
				Key:   key,
				Value: valueNode.Value,
			})
		}
	}
	return result
}
