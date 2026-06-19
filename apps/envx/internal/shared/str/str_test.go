package str

import "testing"

// -------------------------------------------------------------------------------------
// TestDedent verifies common-indent removal, tab expansion, blank-line trimming,
// and the optional preserve-indent argument.
func TestDedent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		indent []int
		want   string
	}{
		{
			name:  "removes common indent",
			input: "\n    a\n    b\n",
			want:  "a\nb",
		},
		{
			name:   "preserves requested indent",
			input:  "\n    a\n    b\n",
			indent: []int{2},
			want:   "  a\n  b",
		},
		{
			name:  "expands tabs",
			input: "\n\tkey: value\n",
			want:  "key: value",
		},
		{
			name:  "keeps nested relative indent",
			input: "\n    a\n      b\n",
			want:  "a\n  b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Dedent(tt.input, tt.indent...)
			if got != tt.want {
				t.Errorf("Dedent() = %q, want %q", got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestContains verifies substring detection including the empty-substring and
// too-long-substring edge cases.
func TestContains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		s, substr string
		want      bool
	}{
		{"hello world", "world", true},
		{"hello", "", true},
		{"hi", "hello", false},
		{"abc", "abc", true},
		{"abc", "d", false},
	}

	for _, tt := range tests {
		if got := Contains(tt.s, tt.substr); got != tt.want {
			t.Errorf("Contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}
