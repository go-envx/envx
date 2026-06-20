package exitcode

import "fmt"

// -------------------------------------------------------------------------------------
// Error wraps a numeric exit code so that main.go can detect it via errors.AsType()
// and call os.Exit() with the exact code carried up from a child process.
type Error struct {
	Code int
}

// -------------------------------------------------------------------------------------
// Error returns a human-readable representation of the exit code.
func (e *Error) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
