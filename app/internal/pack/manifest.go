package pack

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/utils/yamlx"
	"gopkg.in/yaml.v3"
)

// rewriteManifest rewrites the manifest so its paths resolve against the bundle:
// each selected project's includes are replaced with their "<project>/<stem>"
// paths under the project's bundle directory, and any explicit secrets store path
// is dropped so the store resolves to the standardized secrets.yaml at the bundle
// root. Projects that were not selected are removed entirely. It edits the YAML
// node tree in place so comments, key order, and formatting survive.
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
	dropSecretsPath(root)

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

// rewriteIncludes replaces every selected project's include entries with their
// "<project>/<stem>" paths under the project's bundle directory, and drops every
// project that was not selected so the bundle manifest declares only the projects
// it actually carries. An include entry with no assigned name is left untouched.
func rewriteIncludes(root *yaml.Node, bundles []projectBundle) error {
	projects, _ := yamlx.MappingEntry(root, "projects", false)
	if projects == nil || projects.Kind != yaml.MappingNode {
		return errors.New("manifest has no projects mapping")
	}

	byName := make(map[string]projectBundle, len(bundles))
	for _, bundle := range bundles {
		byName[bundle.name] = bundle
	}

	// Walk the project entries back to front so removing an unselected project
	// never shifts an index still to be visited.
	for i := len(projects.Content) - 2; i >= 0; i -= 2 {
		name := projects.Content[i].Value
		bundle, ok := byName[name]
		if !ok {
			// A project the pack did not select carries no files into the bundle, so
			// drop it from the manifest rather than leaving a dangling declaration.
			yamlx.RemoveMappingEntry(projects, i)
			continue
		}
		project := projects.Content[i+1]
		includes, _ := yamlx.MappingEntry(project, "includes", false)
		if includes == nil || includes.Kind != yaml.SequenceNode {
			continue
		}
		for _, entry := range includes.Content {
			if stem, ok := bundle.names[entry.Value]; ok {
				yamlx.SetStringScalar(entry, bundle.dir+"/"+stem)
			}
		}
	}
	return nil
}

// dropSecretsPath removes an explicit secrets store path from the manifest so the
// store resolves to the standardized secrets.yaml beside the bundled manifest,
// which is where pack writes it. It is a no-op when the manifest has no secrets
// block or declares no explicit path. When path was the block's only setting, the
// now-empty secrets block is removed too rather than left as an empty mapping.
func dropSecretsPath(root *yaml.Node) {
	secrets, secretsIndex := yamlx.MappingEntry(root, "secrets", false)
	if secrets == nil || secrets.Kind != yaml.MappingNode {
		return
	}
	if _, pathIndex := yamlx.MappingEntry(secrets, "path", false); pathIndex >= 0 {
		yamlx.RemoveMappingEntry(secrets, pathIndex)
	} else {
		return
	}
	if len(secrets.Content) == 0 {
		yamlx.RemoveMappingEntry(root, secretsIndex)
	}
}
