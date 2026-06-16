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
// Discover locates the manifest file using a three-tier resolution strategy:
//
//  1. Explicit path (from the --config CLI flag) — highest priority.
//  2. ENVX_CONFIG environment variable.
//  3. Walk-up search: starting from cwd, walk parent directories until either
//     envx.yaml is found or the git root / filesystem root is reached.
//
// Returns the absolute path to the manifest or an error if none is found.
func Discover(explicitPath string) (string, error) {
	// 1. Explicit flag takes highest priority.
	if explicitPath != "" {
		clean := filepath.Clean(explicitPath)
		if _, err := os.Stat(clean); err != nil {
			return "", fmt.Errorf("manifest not found at %q: %w", clean, err)
		}
		abs, err := filepath.Abs(clean)
		if err != nil {
			return "", err
		}
		return abs, nil
	}

	// 2. Environment variable.
	if envPath := os.Getenv("ENVX_CONFIG"); envPath != "" {
		clean := filepath.Clean(envPath)
		if _, err := os.Stat(clean); err != nil {
			return "", fmt.Errorf("ENVX_CONFIG=%q: %w", envPath, err)
		}
		abs, err := filepath.Abs(clean)
		if err != nil {
			return "", err
		}
		return abs, nil
	}

	// 3. Walk up from cwd.
	return walkUp()
}

// -------------------------------------------------------------------------------------
// walkUp searches for envx.yaml starting from the current working directory
// and ascending parent directories. The search stops when:
//   - envx.yaml is found (success)
//   - A .git directory is encountered (git root boundary)
//   - The filesystem root is reached (no more parents)
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

		// Stop at git root if present.
		if isGitRoot(dir) {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root.
			break
		}
		dir = parent
	}

	return "", errors.New("envx.yaml not found (searched from cwd to git/filesystem root)")
}

// -------------------------------------------------------------------------------------
// isGitRoot returns true if the given directory contains a .git entry,
// indicating it is the root of a git repository.
func isGitRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
