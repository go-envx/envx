package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/pkg/file"
)

// defaultFilename is the conventional name for the workspace manifest.
const defaultFilename = "envx.yaml"

// -------------------------------------------------------------------------------------

// Load discovers the manifest (an explicit path, else a walk-up search), then
// reads, parses, and validates it. It returns the manifest struct and the
// directory it was loaded from.
func Load(path string) (m *schema.Manifest, dir string, err error) {
	// discover the manifest path (explicit path, else walk-up search)
	found, err := Discover(path)
	if err != nil {
		return nil, "", err
	}
	// read the manifest file from disk
	data, err := os.ReadFile(found) //nolint:gosec // path is user-controlled CLI input
	if err != nil {
		return nil, "", fmt.Errorf("reading manifest: %w", err)
	}
	// parse the manifest file into a schema.Manifest
	m, err = parse(data)
	if err != nil {
		return nil, "", err
	}
	// return the manifest struct and the directory it was loaded from
	return m, filepath.Dir(found), nil
}

// -------------------------------------------------------------------------------------

// parse unmarshals raw YAML into a schema.Manifest and runs structural
// validation. The on-disk location is recorded separately by Load.
func parse(data []byte) (*schema.Manifest, error) {
	var m schema.Manifest
	// unmarshal the YAML into the schema.Manifest struct
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	// validate the manifest's structural constraints
	if err := m.Validate(); err != nil {
		return nil, err
	}
	// return the manifest struct
	return &m, nil
}

// -------------------------------------------------------------------------------------

// Discover locates the manifest file using a two-tier strategy:
//
//  1. An explicit path (already resolved by the caller from the --config flag or
//     ENVX_CONFIG) — highest priority.
//  2. A walk-up search from the working directory until envx.yaml is found or the
//     git/filesystem root is reached.
//
// It returns the absolute path to the manifest or an error if none is found.
func Discover(explicitPath string) (string, error) {
	if explicitPath != "" {
		return resolveExplicit(explicitPath, "manifest not found at %q")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("manifest discovery: %w", err)
	}

	// walk up from the working directory, stopping at the git repository root
	found, err := file.FindUp(cwd, defaultFilename, ".git")
	if errors.Is(err, file.ErrNotFound) {
		return "", errors.New(
			"envx.yaml not found (searched from cwd to git/filesystem root)",
		)
	}
	if err != nil {
		return "", fmt.Errorf("manifest discovery: %w", err)
	}
	return found, nil
}

// -------------------------------------------------------------------------------------

// resolveExplicit validates that a user-provided manifest path exists and
// returns its absolute form. msgFormat carries a single %q verb for the path.
func resolveExplicit(path, msgFormat string) (string, error) {
	clean := filepath.Clean(path)
	if _, err := os.Stat(clean); err != nil {
		return "", fmt.Errorf(msgFormat+": %w", path, err)
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", err
	}
	return abs, nil
}
