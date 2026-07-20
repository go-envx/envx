package create

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-envx/envx/app/pkg/file"
)

// -------------------------------------------------------------------------------------

// templatesFS embeds the create templates. Each subdirectory of templates/ is one
// named workspace ("quick-start") laid out exactly as it should appear on disk.
//
//go:embed all:templates
var templatesFS embed.FS

// Template names, matching the subdirectories under templates/ and the "create"
// subcommand names.
const (
	quickStart = "quick-start"
)

// -------------------------------------------------------------------------------------

// actionParams are the inputs to the create action.
type actionParams struct {
	// Template is the template to scaffold (a subdirectory of templates/).
	Template string
	// TargetDir is the directory to scaffold into.
	TargetDir string
	// Force overwrites existing files instead of stopping on conflicts.
	Force bool
}

// -------------------------------------------------------------------------------------

// actionResult is the data the create action returns.
type actionResult struct {
	// Written lists the destination paths written, in sorted order.
	Written []string
}

// -------------------------------------------------------------------------------------

// execute scaffolds p.Template into p.TargetDir. Without Force it refuses to
// overwrite any existing file, reporting every conflict at once so the caller can
// resolve them before retrying.
func execute(p actionParams) (actionResult, error) {
	root, err := fs.Sub(templatesFS, "templates/"+p.Template)
	if err != nil {
		return actionResult{}, fmt.Errorf("locating template %q: %w", p.Template, err)
	}

	rels, err := collectFiles(root)
	if err != nil {
		return actionResult{}, fmt.Errorf("reading template %q: %w", p.Template, err)
	}

	if !p.Force {
		if err := checkConflicts(p.TargetDir, rels); err != nil {
			return actionResult{}, err
		}
	}

	written := make([]string, 0, len(rels))
	for _, rel := range rels {
		data, err := fs.ReadFile(root, rel)
		if err != nil {
			return actionResult{}, fmt.Errorf("reading template file %q: %w", rel, err)
		}

		dest := filepath.Join(p.TargetDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			return actionResult{}, fmt.Errorf("creating directory for %s: %w", dest, err)
		}
		if err := file.WriteAtomic(dest, data); err != nil {
			return actionResult{}, err
		}
		written = append(written, dest)
	}

	return actionResult{Written: written}, nil
}

// -------------------------------------------------------------------------------------

// collectFiles returns every file path under src, relative to src and slash-separated,
// in sorted order for deterministic output.
func collectFiles(src fs.FS) ([]string, error) {
	var files []string
	err := fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// -------------------------------------------------------------------------------------

// checkConflicts returns an error naming every destination file that already
// exists, so create never clobbers a user's files unless --force is given.
func checkConflicts(targetDir string, rels []string) error {
	var conflicts []string
	for _, rel := range rels {
		dest := filepath.Join(targetDir, filepath.FromSlash(rel))
		switch _, err := os.Stat(dest); {
		case err == nil:
			conflicts = append(conflicts, dest)
		case !os.IsNotExist(err):
			return fmt.Errorf("checking %s: %w", dest, err)
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf(
			"refusing to overwrite %d existing file(s) (use --force):\n  %s",
			len(conflicts), strings.Join(conflicts, "\n  "),
		)
	}
	return nil
}
