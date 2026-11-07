package exitcode

import "fmt"

// Error wraps a numeric exit code and an optional underlying cause.
type Error struct {
	Code  int
	Cause error
}

// Error returns a formatted message describing the exit status and optional cause.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("exit status %d: %v", e.Code, e.Cause)
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

// Unwrap returns the underlying cause of the exit status error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// New returns an error representing the given numeric exit code and underlying cause.
// If code is 0 and cause is nil, New returns nil rather than an Error with code zero,
// ensuring success is represented as nil.
func New(code int, cause error) error {
	if code == 0 && cause == nil {
		return nil
	}
	return &Error{
		Code:  code,
		Cause: cause,
	}
}

// FromCode returns an error representing the given numeric exit code with no
// underlying cause. If code is 0, FromCode returns nil.
func FromCode(code int) error {
	return New(code, nil)
}
