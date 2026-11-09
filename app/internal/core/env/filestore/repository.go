package filestore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/atomicfile"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/lib/git"
)

var (
	_ env.NamespaceReader = Repository{}
	_ env.OverlayWriter   = Repository{}
)

// Repository reads namespace files and writes overlays. The zero value is
// ready to use.
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

// SetOverlay writes text at the dotted key of the overlay under the directory
// lock, preserving comments and layout, and returns the overlay path.
func (Repository) SetOverlay(
	includePath, environment, key, text string,
) (string, error) {
	path := overlayPath(includePath, environment)
	lockPath := filepath.Join(filepath.Dir(path), workspace.LockFilename)
	if err := git.EnsureIgnored(lockPath); err != nil {
		return "", err
	}
	err := atomicfile.ReadModifyWrite(lockPath, path, func(data []byte) ([]byte, error) {
		out, err := setInDocument(data, key, text)
		if err != nil {
			return nil, fmt.Errorf("setting %q in %s: %w", key, path, err)
		}
		return out, nil
	})
	if err != nil {
		return "", err
	}
	return path, nil
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
