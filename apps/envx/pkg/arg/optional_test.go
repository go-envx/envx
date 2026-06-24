package arg

import "testing"

// -------------------------------------------------------------------------------------

// TestOptional verifies in-range access returns the argument while out-of-range
// and negative indices return the empty string.
func TestOptional(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		i    int
		want string
	}{
		{name: "first", args: []string{"a", "b"}, i: 0, want: "a"},
		{name: "present", args: []string{"a", "b"}, i: 1, want: "b"},
		{name: "absent", args: []string{"a"}, i: 1, want: ""},
		{name: "empty", args: nil, i: 0, want: ""},
		{name: "negative", args: []string{"a"}, i: -1, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Optional(tt.args, tt.i); got != tt.want {
				t.Errorf("Optional(%v, %d) = %q, want %q", tt.args, tt.i, got, tt.want)
			}
		})
	}
}
