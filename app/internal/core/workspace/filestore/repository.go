package filestore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/filex"
)

// defaultFilename is the conventional workspace manifest filename.
const defaultFilename = "envx.yaml"

// Params configures filesystem workspace discovery.
type Params struct {
	// Path is an optional explicit manifest file path or directory to start
	// discovery from.
	Path string
	// Filename is the manifest file name to discover (defaults to defaultFilename).
	Filename string
	// WorkDir is where walk-up discovery starts when Path is empty (defaults to
	// the process working directory).
	WorkDir string
}

// Repository loads the workspace manifest from the local filesystem.
type Repository struct {
	path     string
	filename string
	workDir  string
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
		workDir:  params.WorkDir,
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
//  2. A walk-up search from the work directory until the manifest file is found
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

	start := r.workDir
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("manifest discovery: %w", err)
		}
		start = cwd
	}

	found, err := filex.FindUp(start, r.filename, ".git")
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
