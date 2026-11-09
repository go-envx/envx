package env

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrProjectNotFound indicates that a requested project is not declared.
	ErrProjectNotFound = errors.New("project not declared")
	// ErrEnvironmentNotDeclared indicates that a requested environment is not
	// declared.
	ErrEnvironmentNotDeclared = errors.New("environment not declared")
	// ErrOverlayNotFound indicates a required environment overlay file is missing.
	ErrOverlayNotFound = errors.New("required overlay file not found")
	// ErrKeyNotFound indicates a requested variable is not in the environment.
	ErrKeyNotFound = errors.New("key not found")
	// ErrFlattenCollision indicates two distinct object keys flatten to the same
	// variable name.
	ErrFlattenCollision = errors.New("flatten collision detected")
	// ErrCircularReference indicates variable substitution encountered a cycle.
	ErrCircularReference = errors.New("circular variable reference detected")
	// ErrUnresolved indicates a materializing operation found values that did
	// not resolve.
	ErrUnresolved = errors.New("unresolved values")
)

// MissingReferenceError reports a reference that resolves in neither the
// namespace nor the host environment. It carries only names, never values.
type MissingReferenceError struct {
	// Key is the variable whose value holds the dangling reference.
	Key string
	// Reference is the variable name that resolves nowhere.
	Reference string
}

// Error describes the dangling reference without exposing any value.
func (e *MissingReferenceError) Error() string {
	return fmt.Sprintf(
		"variable %q references %q, which is set in neither the namespace "+
			"nor the environment",
		e.Key, e.Reference,
	)
}

// CircularReferenceError reports a reference cycle as an ordered path of
// variable names. It carries only names, never values.
type CircularReferenceError struct {
	// Cycle is the ordered path of variable names, ending where it began.
	Cycle []string
}

// Error describes the cycle as a name path without exposing any value.
func (e *CircularReferenceError) Error() string {
	return "circular reference: " + strings.Join(e.Cycle, " -> ")
}

// Unwrap returns ErrCircularReference.
func (e *CircularReferenceError) Unwrap() error {
	return ErrCircularReference
}
