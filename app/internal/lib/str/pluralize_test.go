package str

import "testing"

// TestPluralize verifies the singular form is used only for a count of one.
func TestPluralize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		n    int
		want string
	}{
		{name: "zero is plural", n: 0, want: "0 values"},
		{name: "one is singular", n: 1, want: "1 value"},
		{name: "many is plural", n: 3, want: "3 values"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Pluralize(tt.n, "value", "values"); got != tt.want {
				t.Errorf("Pluralize(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}
