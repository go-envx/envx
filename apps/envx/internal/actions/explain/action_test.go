package explain

import "testing"

// -------------------------------------------------------------------------------------
// TestMask verifies non-empty values are redacted and empty values are left
// untouched.
func TestMask(t *testing.T) {
	t.Parallel()

	if got := mask("secret"); got != "********" {
		t.Errorf("mask(secret) = %q, want ********", got)
	}
	if got := mask(""); got != "" {
		t.Errorf("mask(empty) = %q, want empty", got)
	}
}
