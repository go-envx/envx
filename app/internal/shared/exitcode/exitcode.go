package exitcode

import "fmt"

// Process exit codes envx returns to the shell. Treat them as public API: scripts
// branch on them, so the values are stable.
const (
	// OK signals successful execution.
	OK = 0
	// Runtime signals a general runtime failure (config, I/O, or resolution).
	Runtime = 1
	// Usage signals an invalid invocation Cobra rejected before the command ran
	// (unknown flags, or bad or missing arguments).
	Usage = 2
)

// Error wraps a numeric exit code so that main.go can detect it via errors.AsType()
// and call os.Exit() with the exact code carried up from a child process.
type Error struct {
	Code int
}

// Error returns a human-readable representation of the exit code.
func (e *Error) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
