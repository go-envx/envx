package exitcode

import (
	"errors"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestErrorMessage verifies the formatted message carries the code.
func TestErrorMessage(t *testing.T) {
	t.Parallel()

	err := &Error{Code: 42}
	if got, want := err.Error(), "exit status 42"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// -------------------------------------------------------------------------------------

// TestErrorAsType verifies the type is recoverable via errors.AsType so main
// can extract the code from a wrapped error — mirroring the consumer path.
func TestErrorAsType(t *testing.T) {
	t.Parallel()

	wrapped := errors.Join(errors.New("context"), &Error{Code: 7})
	ec, ok := errors.AsType[*Error](wrapped)
	if !ok {
		t.Fatal("expected errors.AsType to find *Error")
	}
	if ec.Code != 7 {
		t.Errorf("Code = %d, want 7", ec.Code)
	}
}

// -------------------------------------------------------------------------------------

// TestNew verifies New wraps a code in a recoverable *Error.
func TestNew(t *testing.T) {
	t.Parallel()

	ec, ok := errors.AsType[*Error](New(9))
	if !ok {
		t.Fatalf("expected *Error, got %T", New(9))
	}
	if ec.Code != 9 {
		t.Errorf("Code = %d, want 9", ec.Code)
	}
}
