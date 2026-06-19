package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// -------------------------------------------------------------------------------------
// Load reads, parses, and validates the manifest at path, returning a
// ready-to-use *Config or an error describing what went wrong (file not found,
// parse error, or validation failure).
func Load(path string) (*Config, error) {
	clean := filepath.Clean(path)
	//nolint:gosec // path is user-controlled CLI input; Clean mitigates traversal
	data, err := os.ReadFile(clean)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	return parse(data, filepath.Dir(clean))
}

// -------------------------------------------------------------------------------------
// parse unmarshals raw YAML into a Config, records the workspace root, and runs
// structural validation.
func parse(data []byte, dir string) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	c.dir = dir

	if err := validate(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

// -------------------------------------------------------------------------------------
// validate enforces structural constraints: at least one environment and one
// project must be declared, every project must have at least one include, and
// no include entry may be empty.
func validate(c *Config) error {
	if len(c.Environments) == 0 {
		return errors.New("manifest: environments list must not be empty")
	}
	if len(c.Projects) == 0 {
		return errors.New("manifest: at least one project must be defined")
	}

	for name, p := range c.Projects {
		if len(p.Includes) == 0 {
			return fmt.Errorf("manifest: project %q has no includes", name)
		}
		if slices.Contains(p.Includes, "") {
			return fmt.Errorf("manifest: project %q contains an empty include", name)
		}
	}

	return nil
}
