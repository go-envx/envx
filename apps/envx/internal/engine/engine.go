package engine

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
)

// -------------------------------------------------------------------------------------
// Config is the engine's input contract: a plain data bag the caller fully
// populates (the config package builds it from a manifest plus CLI overrides).
// The engine knows nothing about envx.yaml, cobra, or precedence. Every field
// is optional and the engine fills in terminal defaults itself. The resolved
// knobs live in Settings, the value form the engine merges.
type Config struct {
	Dir          string   // workspace root; include paths resolve against it
	Includes     []string // one project's ordered namespace chain
	Environments []string // declared environments, for validating the target
	Settings     Settings
}

// -------------------------------------------------------------------------------------
// Settings holds the fully-resolved env-resolution knobs the engine consumes — a
// plain value struct with no knowledge of CLI precedence or YAML. Zero values are
// valid: an empty Env falls back to the first declared environment and the
// bool/string knobs default to off. The config package produces it by layering the
// declared schema.Settings over CLI input. It is the effective counterpart to the
// declared schema.Settings.
type Settings struct {
	Env             string
	Strict          bool
	Prefix          string
	Suffix          string
	NamespacePrefix bool
}

// -------------------------------------------------------------------------------------
// Build is the single entry point: it applies the default environment,
// validates it against the declared set, builds the namespace chain from the
// include list, deep-merges them, and returns an immutable Result. It performs no
// precedence resolution and reads no files beyond the namespace overlays.
func Build(c *Config) (*Result, error) {
	if c == nil {
		return nil, errors.New("engine: nil config")
	}

	env := c.Settings.Env
	if env == "" && len(c.Environments) > 0 {
		env = c.Environments[0]
	}
	if !slices.Contains(c.Environments, env) {
		return nil, fmt.Errorf(
			"environment %q is not declared in the manifest (available: %v)",
			env, c.Environments,
		)
	}

	opts := mergeOptions{
		strict:          c.Settings.Strict,
		prefix:          c.Settings.Prefix,
		suffix:          c.Settings.Suffix,
		namespacePrefix: c.Settings.NamespacePrefix,
	}
	return mergeNamespaces(buildNamespaces(c.Dir, c.Includes), env, opts)
}

// -------------------------------------------------------------------------------------
// buildNamespaces resolves each include into an absolute namespace (directory +
// base name), preserving declaration order. Includes are relative to dir.
func buildNamespaces(dir string, includes []string) []namespace {
	out := make([]namespace, 0, len(includes))
	for _, inc := range includes {
		d := filepath.Join(dir, filepath.Dir(inc))
		out = append(out, namespace{dir: d, name: filepath.Base(inc)})
	}
	return out
}
