package cmd

import "fmt"

// ExitCodeError wraps an exit code so that main.go can propagate it from child processes.
type ExitCodeError struct {
	Code int
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
