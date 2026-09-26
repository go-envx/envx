package filestore

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/features/workspace"
	"github.com/go-envx/envx/app/internal/utils/strictyaml"
	"github.com/go-envx/envx/app/internal/utils/yamlx"
)

const (
	// defaultErrorIndent is the number of spaces used to indent follow-up error
	// lines under the printer's "ERROR:" label.
	defaultErrorIndent = 3

	// defaultManifestIndent is the block indentation applied when a manifest document
	// has no detectable nested indentation.
	defaultManifestIndent = 2

	// defaultSuggestionThreshold is the largest edit distance at which a rejected key is
	// still close enough to a valid one to offer as a "did you mean" correction.
	// Three keeps the common typo and the snake/kebab rename in range without
	// suggesting unrelated keys.
	defaultSuggestionThreshold = 3
)

// parseDocument decodes raw YAML manifest data into a Workspace, runs
// structural validation, and detects the document's block indentation.
func parseDocument(data []byte, path string) (*workspace.Workspace, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	var manifest manifestYAML
	if node.Kind != 0 {
		dec := strictyaml.New(
			strictyaml.WithSchema[manifestYAML]("manifest key"),
			strictyaml.WithSchema[settingsYAML]("setting"),
			strictyaml.WithSchema[secretsYAML]("secrets setting"),
			strictyaml.WithSchema[projectYAML]("project key"),
			strictyaml.WithSuggestionThreshold(defaultSuggestionThreshold),
		)
		if err := dec.Decode(data, &manifest); err != nil {
			return nil, formatDecodeError(err)
		}
	}

	indent := defaultManifestIndent
	if detected, ok := yamlx.IndentLevel(&node); ok {
		indent = detected
	}

	ws := manifest.toWorkspace(path, indent)
	if err := ws.Validate(); err != nil {
		return nil, err
	}

	return ws, nil
}

// formatDecodeError translates a strictyaml failure into a manifest-domain error
// aligned with the CLI printer and referencing SchemaDocsURL for unknown keys.
func formatDecodeError(err error) error {
	var decErr *strictyaml.DecodeError
	if !errors.As(err, &decErr) {
		return fmt.Errorf("parsing manifest: %w", err)
	}

	newLine := "\n" + strings.Repeat(" ", defaultErrorIndent)
	capacity := len(decErr.UnknownFields) + len(decErr.OtherErrors) + 1
	lines := make([]string, 0, capacity)
	for _, f := range decErr.UnknownFields {
		msg := fmt.Sprintf("unknown %s %q (line %d)", f.Label, f.Field, f.Line)
		if f.Suggestion != "" {
			msg += fmt.Sprintf("%sDid you mean %q?", newLine, f.Suggestion)
		}
		lines = append(lines, msg)
	}
	lines = append(lines, decErr.OtherErrors...)
	if decErr.HasUnknownFields() {
		docsURL := "See " + workspace.SchemaDocsURL
		lines = append(lines, docsURL)
	}

	return fmt.Errorf("manifest: %s", strings.Join(lines, newLine))
}
