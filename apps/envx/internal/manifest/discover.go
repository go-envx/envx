package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// defaultFilename is the conventional name for the workspace manifest.
const defaultFilename = "envx.yaml"

// -------------------------------------------------------------------------------------
// Discover locates the manifest file using a two-tier strategy:
//
//  1. An explicit path (already resolved by the caller from the --config flag or
//     ENVX_CONFIG) — highest priority.
//  2. A walk-up search from the working directory until envx.yaml is found or the
//     git/filesystem root is reached.
//
// It returns the absolute path to the manifest or an error if none is found.
func Discover(explicitPath string) (string, error) {
	if explicitPath != "" {
		return resolveExplicit(explicitPath, "manifest not found at %q")
	}
	return walkUp()
}

// -------------------------------------------------------------------------------------
// resolveExplicit validates that a user-provided manifest path exists and
// returns its absolute form. msgFormat carries a single %q verb for the path.
func resolveExplicit(path, msgFormat string) (string, error) {
	clean := filepath.Clean(path)
	if _, err := os.Stat(clean); err != nil {
		return "", fmt.Errorf(msgFormat+": %w", path, err)
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", err
	}
	return abs, nil
}

// -------------------------------------------------------------------------------------
// walkUp searches for envx.yaml starting at the current working directory and
// ascending parents. The search stops when the file is found, a .git directory
// marks the repository root, or the filesystem root is reached.
func walkUp() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("manifest discovery: %w", err)
	}

	for {
		candidate := filepath.Join(dir, defaultFilename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		if isGitRoot(dir) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", errors.New(
		"envx.yaml not found (searched from cwd to git/filesystem root)",
	)
}

// -------------------------------------------------------------------------------------
// isGitRoot reports whether dir contains a .git entry, marking the root of a
// git repository (a boundary for the walk-up search).
func isGitRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
