package pack

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/lib/yamlx"
	"gopkg.in/yaml.v3"
)

// defaultIndent is the block indentation for a manifest with none of its own.
const defaultIndent = 2

// rewriteManifest points each selected project's includes at its bundle
// directory, removes unselected projects, and drops an explicit secrets path,
// preserving comments, key order, and formatting.
func rewriteManifest(source []byte, bundles []projectBundle) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(source, &doc); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, errors.New("manifest is empty")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("manifest root is not a mapping")
	}

	if err := rewriteIncludes(root, bundles); err != nil {
		return nil, err
	}
	// The bundled store sits beside the manifest at the default path.
	if entry, found := yamlx.FindMappingEntry(root, "secrets"); found &&
		entry.ValueNode != nil && entry.ValueNode.Kind == yaml.MappingNode {
		block := entry.ValueNode
		if yamlx.DeleteMappingEntry(block, "path") && len(block.Content) == 0 {
			yamlx.DeleteMappingEntry(root, "secrets")
		}
	}

	indent := defaultIndent
	if detected, ok := yamlx.IndentLevel(&doc); ok {
		indent = detected
	}
	out, err := yamlx.Marshal(&doc, indent)
	if err != nil {
		return nil, fmt.Errorf("encoding manifest: %w", err)
	}
	return yamlx.PreserveBlankLines(source, out), nil
}

// rewriteIncludes replaces selected projects' includes with "<dir>/<stem>" and
// deletes projects the bundle does not carry.
func rewriteIncludes(root *yaml.Node, bundles []projectBundle) error {
	projectsEntry, found := yamlx.FindMappingEntry(root, "projects")
	if !found || projectsEntry.ValueNode == nil ||
		projectsEntry.ValueNode.Kind != yaml.MappingNode {
		return errors.New("manifest has no projects mapping")
	}
	projects := projectsEntry.ValueNode

	byName := make(map[string]projectBundle, len(bundles))
	for _, bundle := range bundles {
		byName[bundle.name] = bundle
	}

	// Walk back to front so deleting an entry never shifts one still to visit.
	for i := len(projects.Content) - 2; i >= 0; i -= 2 {
		name := projects.Content[i].Value
		bundle, ok := byName[name]
		if !ok {
			yamlx.DeleteMappingEntry(projects, name)
			continue
		}
		includesEntry, found := yamlx.FindMappingEntry(projects.Content[i+1], "includes")
		if !found || includesEntry.ValueNode == nil ||
			includesEntry.ValueNode.Kind != yaml.SequenceNode {
			continue
		}
		for _, entry := range includesEntry.ValueNode.Content {
			if stem, ok := bundle.names[entry.Value]; ok {
				yamlx.SetScalarNode(entry, bundle.dir+"/"+stem)
			}
		}
	}
	return nil
}
