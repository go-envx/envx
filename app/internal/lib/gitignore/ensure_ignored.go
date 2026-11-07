package gitignore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureIgnored verifies targetPath is ignored by Git or adds a local .gitignore
// rule before sensitive bytes are written. It creates parent directories if needed,
// and leaves ignore files untouched when Git is unavailable or targetPath is already
// ignored by an ancestor ignore rule.
func EnsureIgnored(targetPath string) error {
	if strings.TrimSpace(targetPath) == "" {
		return errors.New("file path is empty")
	}

	dir := filepath.Dir(targetPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	name := filepath.Base(targetPath)
	ignored, available, err := checkIgnore(dir, name)
	if err != nil {
		return err
	}
	if !available || ignored {
		return nil
	}

	ignorePath := filepath.Join(dir, ".gitignore")
	//nolint:gosec // ignorePath is constructed from targetPath directory.
	data, err := os.ReadFile(ignorePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", ignorePath, err)
	}
	if hasIgnoreRule(string(data), name) {
		return nil
	}

	content := string(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += name + "\n"
	if err := writeDurable(ignorePath, []byte(content)); err != nil {
		return fmt.Errorf("protecting file with %s: %w", ignorePath, err)
	}
	return nil
}

// checkIgnore queries Git whether name is ignored within dir.
// The second result reports whether Git was available to answer the query.
func checkIgnore(dir, name string) (ignored, available bool, err error) {
	//nolint:gosec // Git is fixed; name is only a path basename, never a command.
	cmd := exec.CommandContext(
		context.Background(), "git", "check-ignore", "--no-index", "--quiet", "--", name,
	)
	cmd.Dir = dir
	runErr := cmd.Run()
	if runErr == nil {
		return true, true, nil
	}
	var exitError *exec.ExitError
	if errors.As(runErr, &exitError) &&
		(exitError.ExitCode() == 1 || exitError.ExitCode() == 128) {
		return false, true, nil
	}
	if errors.Is(runErr, exec.ErrNotFound) {
		return false, false, nil
	}
	return false, true, fmt.Errorf(
		"checking Git ignore for %s: %w", filepath.Join(dir, name), runErr,
	)
}

// hasIgnoreRule reports whether a local ignore file contains an exact rule for name.
func hasIgnoreRule(content, name string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == name || line == "/"+name {
			return true
		}
	}
	return false
}

// writeDurable writes data to path durably, preserving existing permissions
// or defaulting to 0644. It uses temporary file creation, byte synchronization,
// and atomic rename.
func writeDurable(path string, data []byte) error {
	perm, err := fileMode(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".gitignore-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp file %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("syncing temp file %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting permissions on %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", tmpName, path, err)
	}
	return nil
}

// fileMode returns the existing permission bits for path, or 0644 default.
func fileMode(path string) (os.FileMode, error) {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		return info.Mode().Perm(), nil
	case errors.Is(err, fs.ErrNotExist):
		return 0o644, nil
	default:
		return 0, fmt.Errorf("checking permissions on %s: %w", path, err)
	}
}
