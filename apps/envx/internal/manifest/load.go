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
// Load reads the manifest file at the given path, parses it, and runs
// validation. Returns a ready-to-use Manifest or an error describing what
// went wrong (file not found, parse error, validation failure).
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
// parse unmarshals raw YAML bytes into a Manifest and runs validation.
// dir is set as the workspace root for resolving relative paths.
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
// validate checks structural constraints on the manifest:
//   - At least one environment must be declared.
//   - At least one project must be defined.
//   - Every project must have at least one include.
//   - No include entry may be empty.
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
