package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/utils/filex"
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
	data, err := filex.Read(ignorePath)
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
	if err := filex.WriteAtomic(ignorePath, []byte(content)); err != nil {
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
