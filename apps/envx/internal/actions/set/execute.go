package set

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-envx/envx/apps/envx/internal/shared/file"
	"gopkg.in/yaml.v3"
)

// -------------------------------------------------------------------------------------
// execute is the imperative shell: it resolves the target overlay from the
// include path and the shared environment, reads the current document, applies
// the pure transform, and writes the result back atomically. set never calls
// engine.ResolveEnv — with no project there is nothing to merge, so it uses the
// environment already resolved on the shared context.
func execute(p actionParams, c actionConfig) error {
	cfg := c.Global.Config
	if cfg == nil {
		return errors.New("set: no manifest loaded")
	}

	env := c.Global.Environment
	if !cfg.HasEnvironment(env) {
		return fmt.Errorf(
			"environment %q is not declared in the manifest (available: %v)",
			env, cfg.Environments,
		)
	}

	dir, name, ok := cfg.LookupInclude(p.IncludePath)
	if !ok {
		return fmt.Errorf("include %q not found in manifest", p.IncludePath)
	}
	target := filepath.Join(dir, name+"."+env+".yaml")

	doc, err := readDoc(target)
	if err != nil {
		return err
	}

	out, err := yaml.Marshal(apply(doc, p))
	if err != nil {
		return fmt.Errorf("marshaling yaml: %w", err)
	}
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
