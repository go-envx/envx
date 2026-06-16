package exitcode

import "testing"

// -------------------------------------------------------------------------------------
// TestErrorMessage verifies that Error produces the expected "exit status N"
// string for various exit codes.
func TestErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code int
		want string
	}{
		{code: 0, want: "exit status 0"},
		{code: 1, want: "exit status 1"},
		{code: 127, want: "exit status 127"},
	}

	for _, tt := range tests {
		e := &Error{Code: tt.code}
		if got := e.Error(); got != tt.want {
			t.Errorf("Error{Code: %d}.Error() = %q, want %q", tt.code, got, tt.want)
		}
	}
}
