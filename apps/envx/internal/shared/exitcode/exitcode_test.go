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
// TestErrorAs verifies the type is detectable via errors.As so main can recover
// the code from a wrapped error.
func TestErrorAs(t *testing.T) {
	t.Parallel()

	wrapped := errors.Join(errors.New("context"), &Error{Code: 7})
	var ec *Error
	if !errors.As(wrapped, &ec) {
		t.Fatal("expected errors.As to find *Error")
	}
	if ec.Code != 7 {
		t.Errorf("Code = %d, want 7", ec.Code)
	}
}
