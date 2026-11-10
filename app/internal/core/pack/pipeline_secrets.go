package pack

import (
	"bytes"
	"fmt"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"gopkg.in/yaml.v3"
)

// exportSecrets returns the run-only store holding every value the bundled
// namespaces reference, or nil when none is stored. A reference the store does
// not hold is skipped so it surfaces at run time, as it would unpacked.
func (s *Service) exportSecrets(items []copyItem) ([]byte, error) {
	refs, err := scanReferences(items)
	if err != nil {
		return nil, err
	}

	snapshots := make(map[string]secrets.Snapshot)
	positions := make(map[string]int)
	var groups []secrets.Group
	for _, ref := range refs {
		snapshot, read := snapshots[ref.Group]
		if !read {
			if snapshot, err = s.deps.Secrets.Read(ref.Group); err != nil {
				return nil, fmt.Errorf(
					"reading secrets group %q from %s: %w", ref.Group, s.secretsPath, err,
				)
			}
			snapshots[ref.Group] = snapshot
		}
		stored, found := snapshot.Group.Values[ref.Key]
		if !found {
			continue
		}
		i, grouped := positions[ref.Group]
		if !grouped {
			i = len(groups)
			positions[ref.Group] = i
			groups = append(groups, secrets.Group{
				Name:   snapshot.Group.Name,
				Values: make(map[string]string),
			})
		}
		groups[i].Values[ref.Key] = stored
	}
	if len(groups) == 0 {
		return nil, nil
	}

	var out bytes.Buffer
	if err := s.deps.Secrets.Export(&out, groups); err != nil {
		return nil, fmt.Errorf("exporting bundled secrets: %w", err)
	}
	return out.Bytes(), nil
}

// scanReferences returns the distinct secret references in the bundled files'
// values, in first-seen order. Values that are not references, including
// malformed ones the resolver would reject, are ignored.
func scanReferences(items []copyItem) ([]secrets.Reference, error) {
	seen := make(map[secrets.Reference]struct{})
	var refs []secrets.Reference
	visit := func(raw string) {
		ref, err := secrets.ParseReference(raw, "")
		if err != nil {
			return
		}
		if _, dup := seen[ref]; !dup {
			seen[ref] = struct{}{}
			refs = append(refs, ref)
		}
	}
	scanned := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, dup := scanned[item.src]; dup {
			continue
		}
		scanned[item.src] = struct{}{}
		var root yaml.Node
		if err := yaml.Unmarshal(item.data, &root); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", item.src, err)
		}
		walkValues(&root, visit)
	}
	return refs, nil
}

// walkValues visits every value scalar, skipping mapping keys.
func walkValues(node *yaml.Node, visit func(string)) {
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			walkValues(child, visit)
		}
	case yaml.MappingNode:
		for i := 1; i < len(node.Content); i += 2 {
			walkValues(node.Content[i], visit)
		}
	case yaml.ScalarNode:
		visit(node.Value)
	default:
		// Aliases are scanned where their anchor is defined.
	}
}
