package filestore

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/utils/filex"
	"github.com/go-envx/envx/app/internal/utils/yamlx"
)

var _ env.NamespaceRepository = (*Repository)(nil)

// Params configures namespace file loading.
type Params struct{}

// Repository implements env.NamespaceRepository using local YAML file loading.
type Repository struct{}

// New constructs a file-backed namespace repository.
func New(params Params) *Repository {
	return &Repository{}
}

// LoadBase loads the base namespace tree for an include path.
func (r *Repository) LoadBase(includePath string) (env.NamespaceData, error) {
	clean := strings.TrimSuffix(includePath, ".yaml")
	baseFile := clean + ".yaml"

	data, err := loadYAML(baseFile)
	if err != nil {
		return env.NamespaceData{}, fmt.Errorf("loading base file %s: %w", baseFile, err)
	}

	return env.NamespaceData{
		Data:       data,
		SourcePath: baseFile,
	}, nil
}

// LoadOverlay loads an environment-specific overlay tree, reporting false if absent.
func (r *Repository) LoadOverlay(
	includePath, environment string,
) (env.NamespaceData, bool, error) {
	clean := strings.TrimSuffix(includePath, ".yaml")
	envFile := clean + "." + environment + ".yaml"

	data, err := loadYAML(envFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return env.NamespaceData{SourcePath: envFile}, false, nil
		}
		return env.NamespaceData{}, false, fmt.Errorf(
			"loading environment file %s: %w", envFile, err,
		)
	}

	return env.NamespaceData{
		Data:       data,
		SourcePath: envFile,
	}, true, nil
}

// SetOverlay sets a key-value in an environment overlay file and returns the
// file path.
func (r *Repository) SetOverlay(
	includePath, environment, key, value string,
) (string, error) {
	clean := strings.TrimSuffix(includePath, ".yaml")
	target := clean + "." + environment + ".yaml"

	doc, source, err := readDoc(target)
	if err != nil {
		return "", err
	}

	indent := detectIndent(doc)

	if err := applyNode(doc, key, value); err != nil {
		return "", fmt.Errorf("setting %q in %s: %w", key, target, err)
	}

	out, err := yamlx.Marshal(doc, indent)
	if err != nil {
		return "", fmt.Errorf("marshaling %s: %w", target, err)
	}
	out = yamlx.PreserveBlankLines(source, out)
	if err := filex.WriteAtomic(target, out); err != nil {
		return "", err
	}
	return target, nil
}

// loadYAML reads and unmarshals a YAML file into a generic map. It returns a
// wrapped os.ErrNotExist when the file is missing so callers can distinguish
// "missing" from "malformed".
func loadYAML(path string) (map[string]any, error) {
	data, err := filex.Read(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if m == nil {
		m = make(map[string]any)
	}
	return m, nil
}
