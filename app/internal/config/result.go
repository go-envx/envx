package config

import (
	"fmt"
	"path/filepath"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/secrets"
)

// Result is the aggregate config produces from one manifest load and one
// resolution pass. Actions read the fields they need and ignore the rest;
// OverlayPath derives the set action's target file from the same result.
type Result struct {
	// Envmerge is the constructed envmerge Manager the environment-building actions
	// operate through. ResolveProject builds it; ResolveWorkspace leaves it nil.
	Envmerge *envmerge.Manager

	// Secrets locates the workspace secrets store and private-key file.
	// ResolveProject binds these into the resolver factory the Manager opens on
	// demand; ResolveWorkspace leaves them as data and never reads the store.
	Secrets secrets.Params

	// Cipher contains the configured cipher construction parameters used to
	// compose the secrets manager.
	Cipher cipher.Params

	// defaultEnvironment is the precedence-resolved default environment. OverlayPath
	// reads it to target the set action's overlay file without a Manager.
	defaultEnvironment string

	// manifestContext retains the loaded manifest and its directory so OverlayPath
	// can validate and join a target without re-loading.
	manifestContext
}

// WorkspaceDir returns the absolute directory the manifest was loaded from. The
// explain action uses it to render source paths relative to the workspace root.
func (r *Result) WorkspaceDir() string {
	return r.dir
}

// OverlayPath resolves the absolute path of the overlay file the set action
// writes: <dir>/<include>.<env>.yaml. The environment is the resolved Env with
// the terminal first-declared fallback applied, validated against the declared
// set, and includePath is validated against the manifest before being joined
// against the workspace directory. It targets a single overlay file without
// merging an environment, so it never builds an envmerge result.
func (r *Result) OverlayPath(includePath string) (string, error) {
	env := r.defaultEnvironment
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
