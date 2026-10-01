package syntax

import (
	"errors"
	"fmt"
	"strings"
)

// ErrCircularReference indicates variable substitution encountered a dependency cycle.
var ErrCircularReference = errors.New("circular variable reference detected")

// MissingReferenceError reports a reference that resolves in neither the
// namespace nor the OS environment. It carries only key names, never a value.
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

// CircularReferenceError reports a reference cycle as an ordered path of variable
// names. It carries only names, never a value.
type CircularReferenceError struct {
	// Cycle is the ordered path of variable names, ending where it began.
	Cycle []string
}

// Error describes the cycle as a name path without exposing any value.
func (e *CircularReferenceError) Error() string {
	return "circular reference: " + strings.Join(e.Cycle, " -> ")
}

// Unwrap returns the underlying ErrCircularReference sentinel error.
func (e *CircularReferenceError) Unwrap() error {
	return ErrCircularReference
}

// Is reports whether target matches ErrCircularReference.
func (e *CircularReferenceError) Is(target error) bool {
	return target == ErrCircularReference
}
