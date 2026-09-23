package workspace

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-envx/envx/app/internal/utils/file"
)

// templatesFS embeds the create templates. Each subdirectory of templates/ is one
// named workspace ("quick-start") laid out exactly as it should appear on disk.
//
//go:embed all:templates
var templatesFS embed.FS

// QuickStartTemplate is the template name matching the quick-start subdirectory.
const QuickStartTemplate = "quick-start"

// CreateWorkspaceCommand defines input parameters for scaffolding a workspace.
type CreateWorkspaceCommand struct {
	// Template is the template to scaffold (a subdirectory of templates/).
	Template string
	// TargetDir is the directory to scaffold into.
	TargetDir string
	// Force overwrites existing files instead of stopping on conflicts.
	Force bool
}

// CreateWorkspaceResult represents the result of scaffolding a workspace.
type CreateWorkspaceResult struct {
	// Written lists the destination paths written, in sorted order.
	Written []string
}

// CreateWorkspaceHandler executes the workspace creation use case.
type CreateWorkspaceHandler struct{}

// NewCreateWorkspaceHandler constructs a new CreateWorkspaceHandler.
func NewCreateWorkspaceHandler() *CreateWorkspaceHandler {
	return &CreateWorkspaceHandler{}
}

// Execute scaffolds cmd.Template into cmd.TargetDir. Without Force it refuses to
// overwrite any existing file, reporting every conflict at once so the caller can
// resolve them before retrying.
func (h *CreateWorkspaceHandler) Execute(
	cmd CreateWorkspaceCommand,
) (CreateWorkspaceResult, error) {
	root, err := fs.Sub(templatesFS, "templates/"+cmd.Template)
	if err != nil {
		return CreateWorkspaceResult{}, fmt.Errorf(
			"locating template %q: %w",
			cmd.Template,
			err,
		)
	}

	rels, err := collectFiles(root)
	if err != nil {
		return CreateWorkspaceResult{}, fmt.Errorf(
			"reading template %q: %w",
			cmd.Template,
			err,
		)
	}

	if !cmd.Force {
		if err := checkConflicts(cmd.TargetDir, rels); err != nil {
			return CreateWorkspaceResult{}, err
		}
	}

	written := make([]string, 0, len(rels))
	for _, rel := range rels {
		data, err := fs.ReadFile(root, rel)
		if err != nil {
			return CreateWorkspaceResult{}, fmt.Errorf(
				"reading template file %q: %w",
				rel,
				err,
			)
		}

		dest := filepath.Join(cmd.TargetDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			return CreateWorkspaceResult{}, fmt.Errorf(
				"creating directory for %s: %w",
				dest,
				err,
			)
		}
		if err := file.WriteAtomic(dest, data); err != nil {
			return CreateWorkspaceResult{}, err
		}
		written = append(written, dest)
	}

	return CreateWorkspaceResult{Written: written}, nil
}

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
