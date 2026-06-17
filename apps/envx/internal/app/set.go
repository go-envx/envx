package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/apps/envx/internal/manifest"

	"gopkg.in/yaml.v3"
)

// -------------------------------------------------------------------------------------
// SetInput holds the parameters for the Set command.
type SetInput struct {
	ConfigPath  string
	IncludePath string
	Flags       *manifest.RawFlags
	Changed     manifest.FlagSet
	Key         string
	Value       string
}

// -------------------------------------------------------------------------------------
// Set writes a key-value pair to the environment overlay file for the specified
// include path and resolved environment. It creates the file if it doesn't exist.
// The includePath must match an entry from a project's includes list exactly.
func (a *App) Set(in *SetInput) error {
	m, err := a.LoadManifest(in.ConfigPath)
	if err != nil {
		return err
	}

	// Resolve config to determine the target environment.
	// Use an empty project for resolution since set operates on namespaces.
	emptyProj := &manifest.Project{}
	cfg := a.ConfigResolver.Resolve(in.Flags, in.Changed, m, emptyProj)

	if err := a.ValidateEnvironment(m, cfg.Environment); err != nil {
		return err
	}

	dir, name, ok := m.LookupInclude(in.IncludePath)
	if !ok {
		return fmt.Errorf("include %q not found in manifest", in.IncludePath)
	}

	// Build the target file path: <dir>/<name>.<env>.yaml
	targetFile := filepath.Join(dir, name+"."+cfg.Environment+".yaml")

	// Load existing YAML if the file exists.
	data := make(map[string]any)
	//nolint:gosec // path is derived from validated manifest + user key
	existing, err := os.ReadFile(targetFile)
	if err == nil {
		if err := yaml.Unmarshal(existing, &data); err != nil {
			return fmt.Errorf("parsing %s: %w", targetFile, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", targetFile, err)
	}

	// Set the key, supporting dot-separated paths for nested keys.
	setNestedKey(data, in.Key, in.Value)

	// Marshal and write back.
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling yaml: %w", err)
	}
	if err := os.WriteFile(targetFile, out, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", targetFile, err)
	}

	return nil
}

// -------------------------------------------------------------------------------------
// setNestedKey sets a value in a nested map using a dot-separated key path.
// For example, "credentials.password" creates or updates
// data["credentials"]["password"].
func setNestedKey(data map[string]any, key, value string) {
	parts := strings.Split(key, ".")
	current := data
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part]
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		if m, ok := next.(map[string]any); ok {
			current = m
		} else {
			// Key exists but is not a map — overwrite with a map.
			m := make(map[string]any)
			current[part] = m
			current = m
		}
	}
	current[parts[len(parts)-1]] = value
}
