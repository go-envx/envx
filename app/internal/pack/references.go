package pack

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/secrets"
	"github.com/go-envx/envx/app/pkg/file"
	"gopkg.in/yaml.v3"
)

// scanReferences collects the distinct secret references the bundled namespace
// files make, so pack copies only the secrets the selected environments actually
// use. It reads each file's leaf values and never decrypts; a value that is not a
// secret reference is ignored. Results are returned in first-seen order for
// deterministic output.
func scanReferences(includes []includeFiles) ([]secrets.SecretReference, error) {
	seen := make(map[secrets.SecretReference]struct{})
	var refs []secrets.SecretReference

	add := func(value string) {
		group, key, ok := secrets.ParseReference(value)
		if !ok {
			return
		}
		ref := secrets.SecretReference{Group: group, Key: key}
		if _, dup := seen[ref]; !dup {
			seen[ref] = struct{}{}
			refs = append(refs, ref)
		}
	}

	for _, include := range includes {
		for _, path := range include.sources() {
			if err := scanFile(path, add); err != nil {
				return nil, err
			}
		}
	}
	return refs, nil
}

// sources returns the absolute paths of every file a namespace contributes: its
// base file and each selected overlay.
func (f includeFiles) sources() []string {
	paths := make([]string, 0, 1+len(f.overlays))
	if f.base != "" {
		paths = append(paths, f.base)
	}
	for _, overlay := range f.overlays {
		paths = append(paths, overlay.src)
	}
	return paths
}

// scanFile parses one namespace file and reports every string leaf value to add.
func scanFile(path string, add func(string)) error {
	data, err := file.Read(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	walkValues(&root, add)
	return nil
}

// walkValues visits every value scalar in a YAML node tree, descending into
// sequences and into the value half of each mapping entry. Mapping keys are
// skipped, since a reference is always a value.
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
		// AliasNode and the zero Kind carry no value to scan; the anchor they
		// point at is scanned where it is defined.
	}
}
