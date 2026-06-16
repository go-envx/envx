// Package exitcode provides a shared error type for propagating child process
// exit codes through the application. It exists as a separate package to avoid
// an import cycle between internal/cmd (which registers commands) and
// internal/runner (which executes child processes).
package exitcode

import "fmt"

// -------------------------------------------------------------------------------------
// Error wraps a numeric exit code so that main.go can detect it via errors.As
// and call os.Exit with the exact code from the child process.
type Error struct {
	Code int
}

// -------------------------------------------------------------------------------------
// Error returns a human-readable representation of the exit code.
func (e *Error) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
