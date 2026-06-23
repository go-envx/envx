package set

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/apps/envx/internal/config"
	"github.com/go-envx/envx/apps/envx/internal/file"
	"gopkg.in/yaml.v3"
)

// -------------------------------------------------------------------------------------

// actionParams are the positional inputs to the set action.
type actionParams struct {
	// IncludePath identifies the target overlay from a project's includes list.
	IncludePath string
	// Key is the dot-separated key path to write.
	Key string
	// Value is the value to write at the key path.
	Value string
}

// -------------------------------------------------------------------------------------

// actionConfig holds the set action's config: the shared resolution input. set
// only registers and uses --env (plus the persistent --config) since it mutates a
// single overlay file rather than merging an environment; config.ResolveTarget
// turns the input into the target overlay path.
// actionConfig is the set action's composed config.
type actionConfig struct {
	config.Input
}

// -------------------------------------------------------------------------------------

// execute is the imperative shell: it resolves the target overlay (environment +
// include path) via config, reads the current document, applies the pure
// transform, and writes the result back atomically. set never invokes the engine
// since no project means there is nothing to merge.
func execute(p actionParams, c *actionConfig) error {
	// resolve the target overlay file
	env, dir, name, err := config.ResolveTarget(&c.Input, p.IncludePath)
	if err != nil {
		return err
	}
	target := filepath.Join(dir, name+"."+env+".yaml")

	// read the current document
	doc, err := readDoc(target)
	if err != nil {
		return err
	}

	// apply the change
	out, err := yaml.Marshal(apply(doc, p))
	if err != nil {
		return fmt.Errorf("marshaling yaml: %w", err)
	}

	// write the result back atomically
	return file.WriteAtomic(target, out)
}

// -------------------------------------------------------------------------------------

// readDoc loads an existing overlay into a generic map, returning an empty map
// when the file does not yet exist.
func readDoc(path string) (map[string]any, error) {
	clean := filepath.Clean(path)
	//nolint:gosec // path is derived from the validated manifest, not raw input
	data, err := os.ReadFile(clean)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	doc := make(map[string]any)
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return doc, nil
}

// -------------------------------------------------------------------------------------

// apply is the pure kernel: it returns doc with the key set, creating doc when
// nil. Plain data in, plain data out; no file I/O.
func apply(doc map[string]any, p actionParams) map[string]any {
	if doc == nil {
		doc = make(map[string]any)
	}
	setNestedKey(doc, p.Key, p.Value)
	return doc
}

// -------------------------------------------------------------------------------------

// setNestedKey sets value at a dot-separated key path within data, creating
// intermediate maps as needed and overwriting any non-map node that blocks the
// path.
func setNestedKey(data map[string]any, key, value string) {
	parts := strings.Split(key, ".")
	current := data
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}
