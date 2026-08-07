package manifest

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/file"
)

// defaultFilename is the conventional name for the workspace manifest.
const defaultFilename = "envx.yaml"

// -------------------------------------------------------------------------------------

// Load discovers the manifest (an explicit path, else a walk-up search), then
// reads, parses, and validates it. It returns the manifest struct and the
// absolute path it was loaded from.
func Load(path string) (m *schema.Manifest, manifestPath string, err error) {
	// Discover the manifest path (explicit path, else walk-up search).
	found, err := discover(path)
	if err != nil {
		return nil, "", err
	}
	m, err = load(found)
	return m, found, err
}

// -------------------------------------------------------------------------------------

// LoadOptional loads the manifest when one is available. It returns a nil
// manifest when the explicit or discovered path does not exist, allowing callers
// such as standalone key generation to use application defaults without a
// workspace.
func LoadOptional(path string) (m *schema.Manifest, manifestPath string, err error) {
	manifestPath, err = discover(path)
	if err != nil {
		if errors.Is(err, file.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", err
	}
	m, err = load(manifestPath)
	if err != nil {
		return nil, "", err
	}
	return m, manifestPath, nil
}

// -------------------------------------------------------------------------------------

// load reads and validates a manifest at a path already discovered by the
// loader's discovery step.
func load(found string) (*schema.Manifest, error) {
	// Read the manifest file from disk.
	data, err := file.Read(found)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	// Parse the manifest file into a schema.Manifest.
	m, err := parse(data)
	if err != nil {
		return nil, err
	}

	// Return the parsed manifest.
	return m, nil
}

// -------------------------------------------------------------------------------------

// parse unmarshals raw YAML into a schema.Manifest and runs structural
// validation. The on-disk location is recorded separately by Load.
func parse(data []byte) (*schema.Manifest, error) {
	var m schema.Manifest

	// Unmarshal the YAML into the schema.Manifest struct.
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	// Validate the manifest's structural constraints.
	if err := m.Validate(); err != nil {
		return nil, err
	}

	// Return the manifest struct.
	return &m, nil
}

// -------------------------------------------------------------------------------------

// discover locates the manifest file using a two-tier strategy:
//
//  1. An explicit path is provided via the --config flag or ENVX_CONFIG env variable.
//  2. A walk-up search from the working directory until the manifest file is found
//     or the search reaches the git repository root or filesystem root.
//
// It returns the absolute path to the manifest or an error if none is found.
func discover(explicitPath string) (string, error) {
	// If an explicit path is provided, resolve it to a verified absolute path.
	if explicitPath != "" {
		abs, err := file.AbsExisting(explicitPath)
		if err != nil {
			return "", fmt.Errorf("manifest not found at %q: %w", explicitPath, err)
		}
		return abs, nil
	}

	// No explicit path: walk up from the working directory to find manifest file.
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("manifest discovery: %w", err)
	}

	// Walk up from the working directory, stopping at the git repository root
	found, err := file.FindUp(cwd, defaultFilename, ".git")
	if errors.Is(err, file.ErrNotFound) {
		return "", fmt.Errorf(
			"%s not found (searched from cwd to git/filesystem root): %w",
			defaultFilename,
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
