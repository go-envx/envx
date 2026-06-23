package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/apps/envx/internal/schema"
)

// -------------------------------------------------------------------------------------
// New discovers the manifest path (an explicit path, else a walk-up search) and
// loads it. The caller resolves the --config flag and ENVX_CONFIG precedence into
// path; New does the rest. It is the primary entry point for a ready *Loaded.
func New(path string) (*Loaded, error) {
	found, err := Discover(path)
	if err != nil {
		return nil, err
	}
	return Load(found)
}

// -------------------------------------------------------------------------------------
// Load reads, parses, and validates the manifest at path, returning a
// ready-to-use *Loaded (the parsed schema paired with its workspace dir) or an
// error describing what went wrong (file not found, parse error, or validation
// failure).
func Load(path string) (*Loaded, error) {
	clean := filepath.Clean(path)
	//nolint:gosec // path is user-controlled CLI input; Clean mitigates traversal
	data, err := os.ReadFile(clean)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	m, err := parse(data)
	if err != nil {
		return nil, err
	}
	return &Loaded{Manifest: m, Dir: filepath.Dir(clean)}, nil
}

// -------------------------------------------------------------------------------------
// parse unmarshals raw YAML into a schema.Manifest and runs structural
// validation. The on-disk location is recorded separately by Load.
func parse(data []byte) (*schema.Manifest, error) {
	var m schema.Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	if err := validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// -------------------------------------------------------------------------------------
// validate enforces structural constraints: at least one environment and one
// project must be declared, every project must have at least one include, and
// no include entry may be empty.
func validate(m *schema.Manifest) error {
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
