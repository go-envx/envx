package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// -------------------------------------------------------------------------------------
// New discovers the manifest path (honoring an explicit --config value, then
// ENVX_CONFIG, then a walk-up search) and loads it. It is the primary entry
// point for obtaining a ready-to-use *Manifest.
func New(path string) (*Manifest, error) {
	found, err := Discover(path)
	if err != nil {
		return nil, err
	}
	return Load(found)
}

// -------------------------------------------------------------------------------------
// Load reads, parses, and validates the manifest at path, returning a
// ready-to-use *Manifest or an error describing what went wrong (file not found,
// parse error, or validation failure).
func Load(path string) (*Manifest, error) {
	clean := filepath.Clean(path)
	//nolint:gosec // path is user-controlled CLI input; Clean mitigates traversal
	data, err := os.ReadFile(clean)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	return parse(data, filepath.Dir(clean))
}

// -------------------------------------------------------------------------------------
// parse unmarshals raw YAML into a Manifest, records the workspace root, and runs
// structural validation.
func parse(data []byte, dir string) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	m.dir = dir

	if err := validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// -------------------------------------------------------------------------------------
// validate enforces structural constraints: at least one environment and one
// project must be declared, every project must have at least one include, and
// no include entry may be empty.
func validate(m *Manifest) error {
	if len(m.Environments) == 0 {
		return errors.New("manifest: environments list must not be empty")
	}
	if len(m.Projects) == 0 {
		return errors.New("manifest: at least one project must be defined")
	}

	for name, p := range m.Projects {
		if len(p.Includes) == 0 {
			return fmt.Errorf("manifest: project %q has no includes", name)
		}
		if slices.Contains(p.Includes, "") {
			return fmt.Errorf("manifest: project %q contains an empty include", name)
		}
	}

	return nil
}
