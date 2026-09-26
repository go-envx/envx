package scaffold

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/utils/filex"
)

// TemplatesFS embeds the create templates. Each subdirectory of templates/ is one
// named workspace ("quick-start") laid out exactly as it should appear on disk.
//
//go:embed all:templates
var TemplatesFS embed.FS

// ServiceParams provides template filesystem dependencies to the scaffolding service.
type ServiceParams struct {
	// Source provides template files. Required.
	Source fs.FS
}

// Service coordinates template extraction, conflict checks, and filesystem writes.
type Service struct {
	params ServiceParams
}

// NewService constructs a workspace scaffolding domain service.
func NewService(params ServiceParams) (*Service, error) {
	if params.Source == nil {
		return nil, errors.New("template source fs is required")
	}
	return &Service{params: params}, nil
}

// Create scaffolds a template into the target directory with conflict checks.
func (s *Service) Create(params CreateParams) (CreateResult, error) {
	root, err := fs.Sub(s.params.Source, "templates/"+params.Template)
	if err != nil {
		return CreateResult{}, fmt.Errorf(
			"%w: locating template %q: %w",
			ErrTemplateNotFound,
			params.Template,
			err,
		)
	}

	rels, err := filex.CollectFiles(root)
	if err != nil {
		return CreateResult{}, fmt.Errorf(
			"%w: reading template %q: %w",
			ErrTemplateNotFound,
			params.Template,
			err,
		)
	}
	if len(rels) == 0 {
		return CreateResult{}, fmt.Errorf(
			"%w: template %q has no files",
			ErrTemplateNotFound,
			params.Template,
		)
	}

	if !params.Force {
		if err := s.checkConflicts(params.TargetDir, rels); err != nil {
			return CreateResult{}, err
		}
	}

	written := make([]string, 0, len(rels))
	for _, rel := range rels {
		data, err := fs.ReadFile(root, rel)
		if err != nil {
			return CreateResult{}, fmt.Errorf(
				"reading template file %q: %w",
				rel,
				err,
			)
		}

		dest := filepath.Join(params.TargetDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			return CreateResult{}, fmt.Errorf(
				"creating directory for %s: %w",
				dest,
				err,
			)
		}
		if err := filex.WriteAtomic(dest, data); err != nil {
			return CreateResult{}, err
		}
		written = append(written, dest)
	}

	return CreateResult{Written: written}, nil
}

// checkConflicts returns an error naming every destination file that already
// exists, so create never clobbers a user's files unless --force is given.
func (s *Service) checkConflicts(targetDir string, rels []string) error {
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
			"%w: refusing to overwrite %d existing file(s):\n  %s",
			ErrConflict,
			len(conflicts),
			strings.Join(conflicts, "\n  "),
		)
	}
	return nil
}
