package store

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/utils/file"
	"github.com/go-envx/envx/app/pkg/yamlx"
	"gopkg.in/yaml.v3"
)

// RetainSecrets removes every stored value for which keep returns false, dropping
// any group left empty and the whole secrets block once its last group is
// removed. keep receives each value's group and key exactly as stored. Retained
// value nodes are reused untouched, so their ciphertext and comments survive.
func (d *Document) RetainSecrets(keep func(group, key string) bool) error {
	root, err := d.topLevel()
	if err != nil {
		return err
	}
	entry, err := getMappingEntry(root, secretsField, false)
	if err != nil || !entry.found {
		return err
	}
	secrets := entry.value
	if secrets.Kind != yaml.MappingNode {
		return fmt.Errorf("%s must be a mapping", secretsField)
	}

	// Rebuild the secrets block from the retained groups and entries so index
	// bookkeeping stays trivial; reused nodes carry their comments along.
	keptGroups := make([]*yaml.Node, 0, len(secrets.Content))
	for i := 0; i+1 < len(secrets.Content); i += 2 {
		groupKey := secrets.Content[i]
		groupNode := secrets.Content[i+1]
		group, err := getStringValue(groupKey, "secret group")
		if err != nil || groupNode.Kind != yaml.MappingNode {
			continue
		}

		keptEntries := make([]*yaml.Node, 0, len(groupNode.Content))
		for j := 0; j+1 < len(groupNode.Content); j += 2 {
			key, err := getStringValue(groupNode.Content[j], "secret key")
			if err != nil {
				continue
			}
			if keep(group, key) {
				keptEntries = append(keptEntries, groupNode.Content[j], groupNode.Content[j+1])
			}
		}
		if len(keptEntries) == 0 {
			continue
		}
		groupNode.Content = keptEntries
		keptGroups = append(keptGroups, groupKey, groupNode)
	}

	if len(keptGroups) == 0 {
		yamlx.RemoveMappingEntry(root, entry.index)
		return nil
	}
	secrets.Content = keptGroups
	return nil
}

// RemovePublicKeys drops the entire public_keys block, so a packed run-only
// bundle carries no encryption keys. It is a no-op when the block is absent.
func (d *Document) RemovePublicKeys() error {
	root, err := d.topLevel()
	if err != nil {
		return err
	}
	entry, err := getMappingEntry(root, publicKeysField, false)
	if err != nil {
		return err
	}
	if entry.found {
		yamlx.RemoveMappingEntry(root, entry.index)
	}
	return nil
}

// SaveTo validates and atomically writes the document to path with owner-only
// permissions, mirroring Save but targeting a different file — used to write a
// filtered copy into a bundle without disturbing the source store.
func (d *Document) SaveTo(path string, defaultIndent int) error {
	if path == "" {
		return errors.New("secrets destination path is empty")
	}
	if err := d.validate(); err != nil {
		return fmt.Errorf("validating secrets: %w", err)
	}

	indent := defaultIndent
	if own, ok := yamlx.IndentLevel(&d.root); ok {
		indent = own
	}
	data, err := yamlx.Marshal(&d.root, indent)
	if err != nil {
		return fmt.Errorf("encoding secrets %s: %w", path, err)
	}
	data = yamlx.PreserveBlankLines(d.source, data)
	if err := file.WriteAtomicPrivate(path, data); err != nil {
		return fmt.Errorf("writing secrets %s: %w", path, err)
	}
	return nil
}
