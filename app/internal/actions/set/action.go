package set

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/pkg/file"
	"github.com/go-envx/envx/app/pkg/yamlx"
	"gopkg.in/yaml.v3"
)

// actionParams are the positional inputs to the set action.
type actionParams struct {
	// IncludePath identifies the target overlay from a project's includes list.
	IncludePath string
	// Key is the dot-separated key path to write.
	Key string
	// Value is the value to write at the key path.
	Value string
}

// actionResult carries the written key and overlay location to the renderer.
type actionResult struct {
	// Key is the dot-separated key path that was written.
	Key string
	// OverlayPath is the overlay file that received the value.
	OverlayPath string
}

// execute is the imperative shell: it resolves the target overlay (environment +
// include path) via config, reads the current document into a YAML node tree,
// applies the pure edit, and writes the result back atomically. Editing the node
// tree in place preserves the file's comments, key order, and formatting; set
// never invokes envmerge since no project means there is nothing to merge.
func execute(p actionParams, in *config.Input) (actionResult, error) {
	// resolve the workspace (no project) and derive the target overlay file
	resolved, err := config.ResolveWorkspace(in)
	if err != nil {
		return actionResult{}, err
	}
	target, err := resolved.OverlayPath(p.IncludePath)
	if err != nil {
		return actionResult{}, err
	}

	// read the current document, preserving comments, key order, and formatting
	doc, source, err := readDoc(target)
	if err != nil {
		return actionResult{}, err
	}

	// match the file's existing indentation so the edit blends in
	indent := detectIndent(doc)

	// apply the change surgically to the node tree
	if err := apply(doc, p); err != nil {
		return actionResult{}, fmt.Errorf("setting %q in %s: %w", p.Key, target, err)
	}

	// re-encode and write the result back atomically
	out, err := yamlx.Marshal(doc, indent)
	if err != nil {
		return actionResult{}, fmt.Errorf("marshaling %s: %w", target, err)
	}
	out = yamlx.PreserveBlankLines(source, out)
	if err := file.WriteAtomic(target, out); err != nil {
		return actionResult{}, err
	}
	return actionResult{Key: p.Key, OverlayPath: target}, nil
}

// readDoc parses the overlay at path into a YAML document node, preserving its
// comments, key order, and structure for a surgical edit. It also returns the
// original file content so blank lines can be restored after re-encoding. A
// missing file yields an empty document so the first set creates it.
func readDoc(path string) (*yaml.Node, []byte, error) {
	doc := new(yaml.Node)
	data, err := file.Read(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return doc, nil, nil
		}
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, doc); err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return doc, data, nil
}

// apply is the pure kernel: it sets the key path on the document's root mapping,
// creating the root and any intermediate mappings as needed. It edits value
// nodes in place so surrounding keys, comments, and formatting are left
// untouched. Node tree in, mutated node tree out; no file I/O.
func apply(doc *yaml.Node, p actionParams) error {
	root, err := documentRoot(doc)
	if err != nil {
		return err
	}
	return setNestedKey(root, strings.Split(p.Key, "."), p.Value)
}

// documentRoot returns the mapping node the keys live under, seeding an empty
// mapping for a fresh or empty document. A document whose root is an explicit
// null (an all-comments file) is promoted to a mapping in place so its comments
// survive; any other non-mapping root is rejected as malformed.
func documentRoot(doc *yaml.Node) (*yaml.Node, error) {
	if len(doc.Content) == 0 {
		doc.Kind = yaml.DocumentNode
		root := &yaml.Node{Kind: yaml.MappingNode}
		doc.Content = []*yaml.Node{root}
		return root, nil
	}

	root := doc.Content[0]
	switch {
	case root.Kind == yaml.MappingNode:
		return root, nil
	case root.Kind == yaml.ScalarNode && root.Tag == "!!null":
		root.Kind = yaml.MappingNode
		root.Tag = ""
		root.Value = ""
		return root, nil
	default:
		return nil, errors.New("document root is not a mapping")
	}
}

// setNestedKey walks parts through node's nested mappings, creating intermediate
// mappings as needed, and writes value at the final part. A scalar blocking an
// intermediate step is reinterpreted as a mapping (a first-time write refining a
// bare leaf into a branch). Structured data is never silently discarded: a list
// or mapping the target would overwrite, or a list an intermediate step would
// descend through, is refused with an error so a user's hand-authored YAML
// survives. An existing scalar leaf is updated in place so its comments and
// position survive; a missing key is appended after the existing entries.
func setNestedKey(node *yaml.Node, parts []string, value string) error {
	for i, part := range parts[:len(parts)-1] {
		child, _ := yamlx.MappingEntry(node, part, false)
		switch {
		case child == nil:
			child = &yaml.Node{Kind: yaml.MappingNode}
			appendPair(node, part, child)
		case child.Kind == yaml.MappingNode:
			// Existing branch: descend into it, leaving its entries untouched.
		case child.Kind == yaml.SequenceNode:
			return fmt.Errorf(
				"%q is a list; refusing to overwrite it",
				strings.Join(parts[:i+1], "."),
			)
		default:
			// A scalar (or null) leaf is refined into a mapping so the path can
			// continue; only a single value is superseded, not a collection.
			*child = yaml.Node{Kind: yaml.MappingNode}
		}
		node = child
	}

	last := parts[len(parts)-1]
	if v, _ := yamlx.MappingEntry(node, last, false); v != nil {
		if v.Kind == yaml.SequenceNode || v.Kind == yaml.MappingNode {
			kind := "a mapping"
			if v.Kind == yaml.SequenceNode {
				kind = "a list"
			}
			return fmt.Errorf(
				"%q is %s; refusing to overwrite it", strings.Join(parts, "."), kind,
			)
		}
		yamlx.SetStringScalar(v, value)
		return nil
	}
	leaf := new(yaml.Node)
	yamlx.SetStringScalar(leaf, value)
	appendPair(node, last, leaf)
	return nil
}

// appendPair adds a new key/value entry to the end of a mapping node, leaving the
// order and comments of the existing entries untouched.
func appendPair(node *yaml.Node, key string, value *yaml.Node) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	node.Content = append(node.Content, keyNode, value)
}

// defaultIndent is the indentation width used when a document has no nested
// mapping to infer one from (a flat, empty, or brand-new file).
const defaultIndent = 2

// detectIndent infers the per-level indentation width from the document's first
// nested mapping via yamlx.IndentLevel, falling back to defaultIndent when the
// document has no block indentation to measure.
func detectIndent(doc *yaml.Node) int {
	if n, ok := yamlx.IndentLevel(doc); ok {
		return n
	}
	return defaultIndent
}
