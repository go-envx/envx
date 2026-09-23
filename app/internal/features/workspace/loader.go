package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/internal/utils/file"
	"github.com/go-envx/envx/app/internal/utils/yamlx"
)

// defaultIndent is the block indentation applied when a manifest document has no
// detectable nested indentation.
const defaultIndent = 2

// ManifestLoaderParams supplies the path and discovery filename for a ManifestLoader.
type ManifestLoaderParams struct {
	// Path is the explicit manifest path; an empty value triggers a walk-up search.
	Path string
	// Filename is the conventional manifest filename used for the walk-up search.
	Filename string
}

// LoaderParams is an alias for ManifestLoaderParams.
type LoaderParams = ManifestLoaderParams

// ManifestLoader discovers, reads, parses, and validates the workspace manifest
// bound to one resolved location.
type ManifestLoader struct {
	// params holds the validated construction input privately.
	params ManifestLoaderParams
}

// Loader is an alias for ManifestLoader.
type Loader = ManifestLoader

// NewManifestLoader binds a path and discovery filename into a manifest loader.
// The filename must be supplied by the caller; the path is optional and, when
// empty, triggers a walk-up search for the filename.
func NewManifestLoader(params ManifestLoaderParams) (*ManifestLoader, error) {
	if strings.TrimSpace(params.Filename) == "" {
		return nil, errors.New("manifest filename is empty")
	}
	return &ManifestLoader{params: params}, nil
}

// NewLoader constructs a new ManifestLoader.
func NewLoader(params ManifestLoaderParams) (*ManifestLoader, error) {
	return NewManifestLoader(params)
}

// Exists reports whether a manifest is available at the explicit or discovered
// location. A missing manifest is reported as false without returning an error,
// allowing callers to fall back to application defaults when running outside a
// workspace.
func (m *ManifestLoader) Exists() (bool, error) {
	if _, err := m.discover(); err != nil {
		if errors.Is(err, file.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Discover locates the manifest file (an explicit path, else a walk-up search)
// and returns its absolute path.
func (m *ManifestLoader) Discover() (string, error) {
	return m.discover()
}

// Load discovers the manifest (an explicit path, else a walk-up search), then
// reads, parses, and validates it. It returns the parsed manifest entity.
func (m *ManifestLoader) Load() (*Manifest, error) {
	path, err := m.discover()
	if err != nil {
		return nil, err
	}

	data, err := file.Read(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	manifestDoc, err := m.parse(data)
	if err != nil {
		return nil, err
	}
	manifestDoc.Path = path
	return manifestDoc, nil
}

// parse decodes raw YAML into a schema.Manifest, runs structural validation, and
// detects the document's block indentation before the struct discards
// formatting. The on-disk location is recorded separately by Load.
func (m *ManifestLoader) parse(data []byte) (*Manifest, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	var manifest schema.Manifest
	if node.Kind != 0 {
		decoded, err := strictDecode(data)
		if err != nil {
			return nil, err
		}
		manifest = decoded
	}

	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	indent := defaultIndent
	if detected, ok := yamlx.IndentLevel(&node); ok {
		indent = detected
	}

	return &Manifest{
		Content: &manifest,
		Indent:  indent,
	}, nil
}

// discover locates the manifest file using a two-tier strategy:
//
//  1. An explicit path is provided via the manifest path parameter. A file path
//     is used as-is; a directory path resolves to the conventional manifest
//     filename inside it.
//  2. A walk-up search from the working directory until the manifest file is found
//     or the search reaches the git repository root or filesystem root.
func (m *ManifestLoader) discover() (string, error) {
	if m.params.Path != "" {
		abs, err := file.AbsExisting(m.params.Path)
		if err != nil {
			return "", fmt.Errorf("manifest not found at %q: %w", m.params.Path, err)
		}

		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("manifest not found at %q: %w", m.params.Path, err)
		}
		if info.IsDir() {
			manifestPath, err := file.AbsExisting(filepath.Join(abs, m.params.Filename))
			if err != nil {
				return "", fmt.Errorf(
					"%s not found in directory %q: %w",
					m.params.Filename, m.params.Path, err,
				)
			}
			return manifestPath, nil
		}
		return abs, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("manifest discovery: %w", err)
	}

	found, err := file.FindUp(cwd, m.params.Filename, ".git")
	if errors.Is(err, file.ErrNotFound) {
		return "", fmt.Errorf(
			"%s not found (searched from cwd to git/filesystem root): %w",
			m.params.Filename,
			err,
		)
	}

	if err != nil {
		return "", fmt.Errorf("manifest discovery: %w", err)
	}

	return found, nil
}
