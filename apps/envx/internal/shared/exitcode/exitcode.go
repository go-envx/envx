// Package exitcode provides a shared error type for propagating a numeric exit
// code through the application up to main.go. It exists as its own leaf package
// so any layer (runner, cli, main) can reference the type without creating an
// import cycle.
package exitcode

import "fmt"

// -------------------------------------------------------------------------------------
// Error wraps a numeric exit code so that main.go can detect it via errors.As
// and call os.Exit with the exact code carried up from a child process.
type Error struct {
	Code int
}

// -------------------------------------------------------------------------------------
// Error returns a human-readable representation of the exit code.
func (e *Error) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
