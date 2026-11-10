package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/lib/atomicfile"
	"github.com/go-envx/envx/app/internal/lib/filex"
)

// QuickStartTemplate is the template name matching the quick-start subdirectory.
const QuickStartTemplate = "quick-start"

// Each subdirectory of templates/ is one named workspace laid out exactly as it
// should appear on disk.
//
//go:embed all:templates
var embedded embed.FS

// CreateParams defines input parameters for scaffolding a workspace template.
type CreateParams struct {
	Template  string
	TargetDir string
	Force     bool
}

// CreateResult represents the outcome of scaffolding a workspace template.
type CreateResult struct {
	Written []string
}

// Service extracts workspace templates into a target directory. The zero value
// uses the embedded templates.
type Service struct {
	// Templates replaces the embedded templates; each top-level directory is
	// one template.
	Templates fs.FS
}

// Create scaffolds a template into the target directory with conflict checks.
func (s Service) Create(params CreateParams) (CreateResult, error) {
	root, rels, err := s.template(params.Template)
	if err != nil {
		return CreateResult{}, err
	}

	dests := make([]string, len(rels))
	for i, rel := range rels {
		dest, err := destination(params.TargetDir, rel)
		if err != nil {
			return CreateResult{}, err
		}
		dests[i] = dest
	}

	if !params.Force {
		if err := checkConflicts(dests); err != nil {
			return CreateResult{}, err
		}
	}

	for i, rel := range rels {
		data, err := fs.ReadFile(root, rel)
		if err != nil {
			return CreateResult{}, fmt.Errorf("reading template file %q: %w", rel, err)
		}
		if err := os.MkdirAll(filepath.Dir(dests[i]), 0o750); err != nil {
			return CreateResult{}, fmt.Errorf(
				"creating directory for %s: %w",
				dests[i],
				err,
			)
		}
		if err := atomicfile.Write(dests[i], data); err != nil {
			return CreateResult{}, err
		}
	}

	return CreateResult{Written: dests}, nil
}

// template returns the named template's filesystem and its sorted file paths.
func (s Service) template(name string) (fs.FS, []string, error) {
	source := s.Templates
	if source == nil {
		sub, err := fs.Sub(embedded, "templates")
		if err != nil {
			return nil, nil, fmt.Errorf("opening embedded templates: %w", err)
		}
		source = sub
	}

	if name == "." || strings.Contains(name, "/") || !fs.ValidPath(name) {
		return nil, nil, fmt.Errorf("%w: %q", ErrTemplateNotFound, name)
	}
	root, err := fs.Sub(source, name)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"%w: locating template %q: %w",
			ErrTemplateNotFound,
			name,
			err,
		)
	}

	rels, err := filex.CollectFiles(root)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"%w: reading template %q: %w",
			ErrTemplateNotFound,
			name,
			err,
		)
	}
	if len(rels) == 0 {
		return nil, nil, fmt.Errorf(
			"%w: template %q has no files",
			ErrTemplateNotFound,
			name,
		)
	}
	return root, rels, nil
}

// destination maps a slash-separated template path into targetDir, rejecting
// paths that would land outside it.
func destination(targetDir, rel string) (string, error) {
	local := filepath.FromSlash(rel)
	if !filepath.IsLocal(local) {
		return "", fmt.Errorf("%w: %q", ErrUnsafePath, rel)
	}
	return filepath.Join(targetDir, local), nil
}

// checkConflicts returns an error naming every destination file that already
// exists, so create never clobbers a user's files unless forced.
func checkConflicts(dests []string) error {
	var conflicts []string
	for _, dest := range dests {
		switch _, err := os.Stat(dest); {
		case err == nil:
			conflicts = append(conflicts, dest)
		case !os.IsNotExist(err):
			return fmt.Errorf("checking %s: %w", dest, err)
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf(
			"%w: refusing to overwrite %d existing file(s):\n  %s",
			ErrConflict,
			len(conflicts),
			strings.Join(conflicts, "\n  "),
		)
	}
	return nil
}
