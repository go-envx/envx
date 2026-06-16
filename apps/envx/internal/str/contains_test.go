package str

import "testing"

// -------------------------------------------------------------------------------------
// TestContains verifies that Contains correctly identifies whether a substring
// exists within a given string, handling edge cases such as empty strings,
// exact matches, and non-matching inputs.
func TestContains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{
			name:   "exact match",
			s:      "hello",
			substr: "hello",
			want:   true,
		},
		{
			name:   "prefix match",
			s:      "hello world",
			substr: "hello",
			want:   true,
		},
		{
			name:   "suffix match",
			s:      "hello world",
			substr: "world",
			want:   true,
		},
		{
			name:   "middle match",
			s:      "hello world",
			substr: "lo wo",
			want:   true,
		},
		{
			name:   "no match",
			s:      "hello world",
			substr: "xyz",
			want:   false,
		},
		{
			name:   "empty substr always matches",
			s:      "hello",
			substr: "",
			want:   true,
		},
		{
			name:   "empty s with empty substr",
			s:      "",
			substr: "",
			want:   true,
		},
		{
			name:   "empty s with non-empty substr",
			s:      "",
			substr: "a",
			want:   false,
		},
		{
			name:   "substr longer than s",
			s:      "hi",
			substr: "hello",
			want:   false,
		},
		{
			name:   "single character match",
			s:      "abc",
			substr: "b",
			want:   true,
		},
		{
			name:   "single character no match",
			s:      "abc",
			substr: "z",
			want:   false,
		},
		{
			name:   "repeated pattern matches first",
			s:      "abcabc",
			substr: "cab",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("Contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}
