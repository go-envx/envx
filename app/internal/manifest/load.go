package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/schema"
)

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
