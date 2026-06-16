package file

import (
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------------------------
// TestWriteCreatesFile verifies that Write creates a file with the expected
// content and that indented backtick strings are automatically dedented.
func TestWriteCreatesFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "plain string",
			content: "key: value",
			want:    "key: value",
		},
		{
			name: "indented backtick string",
			content: `
				host: localhost
				port: 5432
			`,
			want: "host: localhost\nport: 5432",
		},
		{
			name:    "empty string",
			content: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()

			Write(t, dir, "test.yaml", tt.content)

			got, err := os.ReadFile(filepath.Join(dir, "test.yaml")) //nolint:gosec // test code
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("file content:\n got: %q\nwant: %q", string(got), tt.want)
			}
		})
	}
}
