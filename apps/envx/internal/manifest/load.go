package manifest

import (
	"fmt"
	"os"
	"path/filepath"

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

	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}
