package arg

import "testing"

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
		{name: "first", args: []string{"a", "b", "c"}, i: 0, want: "a"},
		{name: "middle", args: []string{"a", "b", "c"}, i: 1, want: "b"},
		{name: "last", args: []string{"a", "b", "c"}, i: 2, want: "c"},
		{name: "absent beyond length", args: []string{"a"}, i: 1, want: ""},
		{name: "absent far out of bounds", args: []string{"a"}, i: 10, want: ""},
		{name: "nil slice", args: nil, i: 0, want: ""},
		{name: "empty slice", args: []string{}, i: 0, want: ""},
		{name: "negative index", args: []string{"a"}, i: -1, want: ""},
		{name: "large negative index", args: []string{"a"}, i: -10, want: ""},
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
