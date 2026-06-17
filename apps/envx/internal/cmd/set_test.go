package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/file"
)

// -------------------------------------------------------------------------------------
// TestSetCommand verifies the Cobra wiring of "envx set": argument validation
// and basic integration. Business logic is tested in internal/app/set_test.go.
func TestSetCommand(t *testing.T) {
	t.Parallel()

	// Create a temporary manifest for the integration case.
	dir := t.TempDir()

	file.Write(t, dir, "envx.yaml", `
		environments: [development, staging]
		projects:
		  api-core:
		    includes:
		      - env/postgres
		      - apps/api-core/env/api-core
	`)

	envDir := filepath.Join(dir, "env")
	if err := os.MkdirAll(envDir, 0o750); err != nil {
		t.Fatal(err)
	}
	file.Write(t, envDir, "postgres.yaml", "host: localhost")

	appDir := filepath.Join(dir, "apps", "api-core", "env")
	if err := os.MkdirAll(appDir, 0o750); err != nil {
		t.Fatal(err)
	}
	file.Write(t, appDir, "api-core.yaml", "port: 3000")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "set succeeds through Cobra",
			args: []string{
				"set", "--config", filepath.Join(dir, "envx.yaml"),
				"--env", "development",
				"env/postgres", "host", "new-db.local",
			},
		},
		{
			name:    "missing args",
			args:    []string{"set", "env/postgres", "key"},
			wantErr: "accepts 3 arg(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := execCmd(tt.args...)

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
		})
	}
}
