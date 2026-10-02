package filex

import (
	"path/filepath"
	"testing"
)

// TestRelativeTo verifies paths inside the base become relative and paths
// outside it are returned unchanged.
func TestRelativeTo(t *testing.T) {
	t.Parallel()

	sep := string(filepath.Separator)
	base := sep + "workspace"
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "inside",
			path: filepath.Join(base, "env", "postgres.yaml"),
			want: filepath.Join("env", "postgres.yaml"),
		},
		{
			name: "dotted name inside",
			path: filepath.Join(base, "..env", "postgres.yaml"),
			want: filepath.Join("..env", "postgres.yaml"),
		},
		{
			name: "outside",
			path: sep + filepath.Join("other", "file.yaml"),
			want: sep + filepath.Join("other", "file.yaml"),
		},
		{
			name: "parent",
			path: sep,
			want: sep,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := RelativeTo(base, tt.path); got != tt.want {
				t.Errorf("RelativeTo(%q, %q) = %q, want %q", base, tt.path, got, tt.want)
			}
		})
	}
}
