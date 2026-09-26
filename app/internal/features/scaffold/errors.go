package scaffold

import "errors"

var (
	// ErrTemplateNotFound indicates that an unknown starter template was requested.
	ErrTemplateNotFound = errors.New("template not found")
	// ErrConflict indicates existing files conflict with template scaffolding.
	ErrConflict = errors.New(
		"target directory contains conflicting files (use --force to overwrite)",
	)
)
