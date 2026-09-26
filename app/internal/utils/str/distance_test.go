package str_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/utils/str"
)

func TestLevenshtein(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "identical", a: "kitten", b: "kitten", want: 0},
		{name: "empty both", a: "", b: "", want: 0},
		{name: "empty a", a: "", b: "abc", want: 3},
		{name: "empty b", a: "abc", b: "", want: 3},
		{name: "substitution", a: "kitten", b: "sitten", want: 1},
		{name: "classic", a: "kitten", b: "sitting", want: 3},
		{name: "unicode runes", a: "résumé", b: "resume", want: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := str.Levenshtein(tc.a, tc.b); got != tc.want {
				t.Errorf("Levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestClosest(t *testing.T) {
	t.Parallel()

	candidates := []string{
		"environment",
		"environments",
		"projects",
		"settings",
		"secrets",
	}

	tests := []struct {
		name        string
		target      string
		candidates  []string
		maxDistance int
		wantMatch   string
		wantFound   bool
	}{
		{
			name:        "exact match",
			target:      "settings",
			candidates:  candidates,
			maxDistance: 3,
			wantMatch:   "settings",
			wantFound:   true,
		},
		{
			name:        "within distance",
			target:      "setting",
			candidates:  candidates,
			maxDistance: 3,
			wantMatch:   "settings",
			wantFound:   true,
		},
		{
			name:        "typo within distance",
			target:      "secretz",
			candidates:  candidates,
			maxDistance: 3,
			wantMatch:   "secrets",
			wantFound:   true,
		},
		{
			name:        "beyond max distance",
			target:      "completelyunrelated",
			candidates:  candidates,
			maxDistance: 3,
			wantMatch:   "",
			wantFound:   false,
		},
		{
			name:        "empty candidates",
			target:      "settings",
			candidates:  nil,
			maxDistance: 3,
			wantMatch:   "",
			wantFound:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotMatch, gotFound := str.Closest(tc.target, tc.candidates, tc.maxDistance)
			if gotFound != tc.wantFound || gotMatch != tc.wantMatch {
				t.Errorf(
					"Closest(%q, %v, %d) = (%q, %v), want (%q, %v)",
					tc.target,
					tc.candidates,
					tc.maxDistance,
					gotMatch,
					gotFound,
					tc.wantMatch,
					tc.wantFound,
				)
			}
		})
	}
}
