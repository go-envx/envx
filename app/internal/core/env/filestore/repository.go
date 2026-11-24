package filestore

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/lib/filex"
)

var _ env.NamespaceReader = Repository{}

// Repository reads namespace files. The zero value is ready to use.
type Repository struct{}

// LoadBase loads <include>.yaml.
func (Repository) LoadBase(includePath string) (env.NamespaceData, error) {
	path := strings.TrimSuffix(includePath, ".yaml") + ".yaml"
	data, err := loadYAML(path)
	if err != nil {
		return env.NamespaceData{}, fmt.Errorf("loading base file %s: %w", path, err)
	}
	return env.NamespaceData{Data: data, SourcePath: path}, nil
}

// LoadOverlay loads <include>.<environment>.yaml, reporting false with the
// expected path when it does not exist.
func (Repository) LoadOverlay(
	includePath, environment string,
) (env.NamespaceData, bool, error) {
	path := overlayPath(includePath, environment)
	data, err := loadYAML(path)
	if errors.Is(err, os.ErrNotExist) {
		return env.NamespaceData{SourcePath: path}, false, nil
	}
	if err != nil {
		return env.NamespaceData{}, false, fmt.Errorf(
			"loading environment file %s: %w", path, err,
		)
	}
	return env.NamespaceData{Data: data, SourcePath: path}, true, nil
}

// overlayPath returns <include>.<environment>.yaml.
func overlayPath(includePath, environment string) string {
	return strings.TrimSuffix(includePath, ".yaml") + "." + environment + ".yaml"
}

// loadYAML reads a YAML mapping; an empty file is an empty map. A missing file
// wraps os.ErrNotExist.
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
