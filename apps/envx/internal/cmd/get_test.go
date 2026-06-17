package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------
// TestGetCommand verifies the Cobra wiring of "envx get": argument validation
// and output formatting. Business logic is tested in internal/app/get_test.go.
func TestGetCommand(t *testing.T) {
	t.Parallel()

	fixtureDir := testdataDir(t, "basic")

	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr string
	}{
		{
			name: "outputs value with trailing newline",
			args: []string{
				"get", "--config", filepath.Join(fixtureDir, "envx.yaml"),
				"--env", "development",
				"api-core", "HOST",
			},
			want: "dev-db.local",
		},
		{
			name:    "missing args",
			args:    []string{"get", "api-core"},
			wantErr: "accepts 2 arg(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout, _, err := execCmd(tt.args...)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := strings.TrimSpace(stdout.String())
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
