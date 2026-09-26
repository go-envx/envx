package workspace

import "errors"

var (
	// ErrNotFound indicates that no workspace configuration could be located.
	ErrNotFound = errors.New("workspace not found")
	// ErrProjectNotFound indicates that a requested project is not declared.
	ErrProjectNotFound = errors.New("project not declared in workspace")
	// ErrEnvironmentNotFound indicates an undeclared target environment.
	ErrEnvironmentNotFound = errors.New("environment not declared in workspace")
)
