package config

import (
	"fmt"
	"path/filepath"

	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/runner"
	"github.com/go-envx/envx/app/internal/secrets"
)

// -------------------------------------------------------------------------------------

// Result is the aggregate config produces from one manifest load and one
// resolution pass. Actions read the fields they need and ignore the rest;
// OverlayPath derives the set action's target file from the same result.
type Result struct {
	// Envmerge is the resolved envmerge input for actions that merge an environment.
	Envmerge envmerge.Params

	// Runner is the resolved runner input for actions that run a command. Config
	// resolves only the Overload knob (flag > ENVX_OVERLOAD > project > global); the
	// run action supplies the merged Env and output streams before invoking runner.
	Runner runner.Params

	// Secrets locates the workspace secrets store. ResolveProject opens it and
	// wires the value resolver; ResolveWorkspace leaves it as data and never reads
	// the store.
	Secrets secrets.Settings

	// manifestContext retains the loaded manifest and its directory so OverlayPath
	// can validate and join a target without re-loading.
	manifestContext
}

// -------------------------------------------------------------------------------------

// OverlayPath resolves the absolute path of the overlay file the set action
// writes: <dir>/<include>.<env>.yaml. The environment is the resolved Env with
// the terminal first-declared fallback applied, validated against the declared
// set, and includePath is validated against the manifest before being joined
// against the workspace directory. It targets a single overlay file without
// merging an environment, so it never builds an envmerge result.
func (r *Result) OverlayPath(includePath string) (string, error) {
	env := r.Envmerge.Settings.Env
	if env == "" {
		env = r.manifest.DefaultEnvironment()
	}
	if !r.manifest.HasEnvironment(env) {
		return "", fmt.Errorf(
			"environment %q is not declared in the manifest (available: %v)",
			env, r.manifest.Environments,
		)
	}
	if !r.manifest.HasInclude(includePath) {
		return "", fmt.Errorf("include %q not found in manifest", includePath)
	}
	return filepath.Join(r.dir, includePath) + "." + env + ".yaml", nil
}
