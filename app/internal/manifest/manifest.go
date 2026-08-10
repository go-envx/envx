package manifest

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/file"
)

// Params supplies the path and discovery filename for a Manager.
type Params struct {
	// Path is the explicit manifest path; an empty value triggers a walk-up search.
	Path string
	// Filename is the conventional manifest filename used for the walk-up search.
	Filename string
}

// Manager discovers, reads, parses, and validates the workspace manifest bound
// to one resolved location.
type Manager struct {
	// params holds the validated construction input privately.
	params Params
}

// New binds a path and discovery filename into a manager. The filename must be
// supplied by the configuration layer; the path is optional and, when empty,
// triggers a walk-up search for the filename.
func New(params Params) (*Manager, error) {
	// Require the discovery filename from the configuration layer.
	if strings.TrimSpace(params.Filename) == "" {
		return nil, errors.New("manifest filename is empty")
	}

	// Assemble the manager with the validated construction input.
	return &Manager{params: params}, nil
}

// Exists reports whether a manifest is available at the explicit or discovered
// location. A missing manifest is reported as false without becoming an error,
// allowing callers such as standalone key generation to use application defaults
// without a workspace.
func (m *Manager) Exists() (bool, error) {
	// Discovery locates the manifest without reading it.
	if _, err := m.discover(); err != nil {
		if errors.Is(err, file.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Load discovers the manifest (an explicit path, else a walk-up search), then
// reads, parses, and validates it. It returns the manifest struct and the
// absolute path it was loaded from.
func (m *Manager) Load() (manifest *schema.Manifest, path string, err error) {
	// Discover the manifest path (explicit path, else walk-up search).
	found, err := m.discover()
	if err != nil {
		return nil, "", err
	}
	manifest, err = m.load(found)
	return manifest, found, err
}

// load reads and validates a manifest at a path already located by the discovery
// step.
func (m *Manager) load(path string) (*schema.Manifest, error) {
	// Read the manifest file from disk.
	data, err := file.Read(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	// Parse and validate the manifest file into a schema.Manifest.
	return m.parse(data)
}

// parse unmarshals raw YAML into a schema.Manifest and runs structural
// validation. The on-disk location is recorded separately by Load.
func (m *Manager) parse(data []byte) (*schema.Manifest, error) {
	var manifest schema.Manifest

	// Unmarshal the YAML into the schema.Manifest struct.
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	// Validate the manifest's structural constraints.
	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	// Return the parsed manifest.
	return &manifest, nil
}

// discover locates the manifest file using a two-tier strategy:
//
//  1. An explicit path is provided via the --config flag or ENVX_CONFIG env variable.
//  2. A walk-up search from the working directory until the manifest file is found
//     or the search reaches the git repository root or filesystem root.
//
// It returns the absolute path to the manifest or an error if none is found.
func (m *Manager) discover() (string, error) {
	// If an explicit path is provided, resolve it to a verified absolute path.
	if m.params.Path != "" {
		abs, err := file.AbsExisting(m.params.Path)
		if err != nil {
			return "", fmt.Errorf("manifest not found at %q: %w", m.params.Path, err)
		}
		return abs, nil
	}

	// No explicit path: walk up from the working directory to find manifest file.
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("manifest discovery: %w", err)
	}

	// Walk up from the working directory, stopping at the git repository root
	found, err := file.FindUp(cwd, m.params.Filename, ".git")
	if errors.Is(err, file.ErrNotFound) {
		return "", fmt.Errorf(
			"%s not found (searched from cwd to git/filesystem root): %w",
			m.params.Filename,
			err,
		)
	}

	// Any other error is unexpected and should be reported.
	if err != nil {
		return "", fmt.Errorf("manifest discovery: %w", err)
	}

	// Return the absolute path to the discovered manifest file.
	return found, nil
}
