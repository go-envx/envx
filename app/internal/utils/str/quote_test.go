package str

import "testing"

// TestQuotePath verifies a path is quoted only when it contains a space.
func TestQuotePath(t *testing.T) {
	t.Parallel()

	if got := QuotePath("/no/space"); got != "/no/space" {
		t.Errorf("QuotePath() = %q, want the raw path", got)
	}

	spaced := "/with space/file.yaml"
	if got := QuotePath(spaced); got != `"`+spaced+`"` {
		t.Errorf("QuotePath() = %q, want a quoted path", got)
	}
}
