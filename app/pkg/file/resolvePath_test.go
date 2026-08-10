package file

import (
	"path/filepath"
	"testing"
)

// TestResolvePath verifies relative paths resolve against the base and absolute
// paths remain rooted at their own location.
func TestResolvePath(t *testing.T) {
	t.Parallel()

	base := filepath.Join(string(filepath.Separator), "workspace")
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "relative",
			path: filepath.Join("private", "secrets.yaml"),
			want: filepath.Join(base, "private", "secrets.yaml"),
		},
		{
			name: "absolute",
			path: filepath.Join(
				string(filepath.Separator),
				"other",
				"store",
				"..",
				"secrets.yaml",
			),
			want: filepath.Join(
				string(filepath.Separator),
				"other",
				"secrets.yaml",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := ResolvePath(base, test.path); got != test.want {
				t.Errorf("ResolvePath(%q, %q) = %q, want %q", base, test.path, got, test.want)
			}
		})
	}
}
