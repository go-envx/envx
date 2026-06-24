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

// -------------------------------------------------------------------------------------

// New wraps a numeric exit code in an *Error. Its signature matches the exit-code
// mapper that runner.Options.ExitError expects, so it can be passed directly.
func New(code int) error {
	return &Error{Code: code}
}
