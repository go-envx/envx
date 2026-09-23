package privatekey

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDestinationsWriteWithoutReturningMaterial verifies file mode, file format,
// and explicit writer handoff behavior.
func TestDestinationsWriteWithoutReturningMaterial(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "envx.keys")
	if err := NewFileDestination(path).Write("production", "private-value"); err != nil {
		t.Fatalf("file destination Write(): %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("private key mode = %o, want 600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is test-local.
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "PRODUCTION=private-value\n" {
		t.Errorf("file contents = %q", data)
	}

	var output bytes.Buffer
	if err := NewWriterDestination(&output).Write(
		"production", "private-value",
	); err != nil {
		t.Fatalf("writer destination Write(): %v", err)
	}
	if !strings.Contains(output.String(), "PRODUCTION=private-value\n") {
		t.Errorf("writer output = %q", output.String())
	}
}

// TestFilePath reports paths only for destinations that expose one.
func TestFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		destination Destination
		want        string
		wantPresent bool
	}{
		{
			name:        "file destination",
			destination: NewFileDestination("envx.keys"),
			want:        "envx.keys",
			wantPresent: true,
		},
		{
			name:        "writer destination",
			destination: NewWriterDestination(&bytes.Buffer{}),
			wantPresent: false,
		},
		{name: "nil destination", wantPresent: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, present := FilePath(tt.destination)
			if got != tt.want || present != tt.wantPresent {
				t.Errorf(
					"FilePath() = (%q, %t), want (%q, %t)",
					got, present, tt.want, tt.wantPresent,
				)
			}
		})
	}
}
