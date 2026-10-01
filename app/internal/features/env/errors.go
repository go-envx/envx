package env

import (
	"errors"

	"github.com/go-envx/envx/app/internal/features/env/syntax"
)

var (
	// ErrEnvironmentNotDeclared indicates that a requested environment is not declared.
	ErrEnvironmentNotDeclared = errors.New("environment not declared")
	// ErrOverlayNotFound indicates a required environment overlay file is missing.
	ErrOverlayNotFound = errors.New("required overlay file not found")
	// ErrFlattenCollision indicates two distinct object keys flatten to the same
	// variable name.
	ErrFlattenCollision = errors.New("flatten collision detected")
	// ErrCircularReference indicates variable substitution encountered a dependency cycle.
	ErrCircularReference = syntax.ErrCircularReference
)
