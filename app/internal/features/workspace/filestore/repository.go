package filestore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/features/workspace"
	"github.com/go-envx/envx/app/internal/utils/filex"
)

var _ workspace.Repository = (*Repository)(nil)

// defaultFilename is the conventional workspace manifest filename.
const defaultFilename = "envx.yaml"

// Params configures filesystem workspace discovery.
type Params struct {
	// Path is an optional explicit manifest file path or directory to start
	// discovery from.
	Path string
	// Filename is the manifest file name to discover (defaults to defaultFilename).
	Filename string
}

// Repository implements workspace.Repository for the local filesystem using
// YAML decoding.
type Repository struct {
	path     string
	filename string
}

// New constructs a file-backed workspace repository.
func New(params Params) (*Repository, error) {
	filename := strings.TrimSpace(params.Filename)
	if filename == "" {
		filename = defaultFilename
	}
	return &Repository{
		path:     params.Path,
		filename: filename,
	}, nil
}

// Load reads the workspace file, applies strict YAML decoding, and returns
// the domain entity.
func (r *Repository) Load() (*workspace.Workspace, error) {
	path, err := r.discover()
	if err != nil {
		return nil, err
	}

	data, err := filex.Read(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	return parseDocument(data, path)
}

// discover locates the manifest file using a two-tier strategy:
//
//  1. An explicit path is provided via the manifest path parameter. A file path
//     is used as-is; a directory path resolves to the conventional manifest
//     filename inside it.
//  2. A walk-up search from the working directory until the manifest file is found
//     or the search reaches the git repository root or filesystem root.
func (r *Repository) discover() (string, error) {
	if r.path != "" {
		abs, err := filex.AbsExisting(r.path)
		if err != nil {
			return "", fmt.Errorf(
				"%w: manifest not found at %q: %w",
				workspace.ErrNotFound,
				r.path,
				err,
			)
		}

		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf(
				"%w: manifest not found at %q: %w",
				workspace.ErrNotFound,
				r.path,
				err,
			)
		}
		if info.IsDir() {
			manifestPath, err := filex.AbsExisting(filepath.Join(abs, r.filename))
			if err != nil {
				return "", fmt.Errorf(
					"%w: %s not found in directory %q: %w",
					workspace.ErrNotFound, r.filename, r.path, err,
				)
			}
			return manifestPath, nil
		}
		return abs, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("manifest discovery: %w", err)
	}

	found, err := filex.FindUp(cwd, r.filename, ".git")
	if errors.Is(err, filex.ErrNotFound) {
		return "", fmt.Errorf(
			"%w: %s not found (searched from cwd to git/filesystem root): %w",
			workspace.ErrNotFound, r.filename, err,
		)
	}
	if err != nil {
		return "", fmt.Errorf("manifest discovery: %w", err)
	}

	return found, nil
}
