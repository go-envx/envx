package filestore_test

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/core/workspace"
)

// loadLocalEnvPath loads a minimal manifest with extra top-level YAML.
func loadLocalEnvPath(t *testing.T, extra string) *workspace.Workspace {
	t.Helper()
	path := writeManifest(t, minimalManifest+"\n"+extra)
	ws, err := newRepository(t, path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return ws
}

// TestLocalEnvPath verifies the default and the manifest key.
func TestLocalEnvPath(t *testing.T) {
	t.Parallel()

	abs := filepath.Join(t.TempDir(), "machine.env")
	tests := []struct {
		name  string
		extra string
		want  func(root string) string
	}{
		{"default", "", func(root string) string {
			return filepath.Join(root, "envx.local.env")
		}},
		{
			"relative manifest key", "localEnvPath: conf/local.env\n",
			func(root string) string { return filepath.Join(root, "conf", "local.env") },
		},
		{
			"absolute manifest key", "localEnvPath: " + abs + "\n",
			func(string) string { return abs },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ws := loadLocalEnvPath(t, tt.extra)
			if want := tt.want(ws.Root); ws.LocalEnvPath != want {
				t.Errorf("LocalEnvPath = %q, want %q", ws.LocalEnvPath, want)
			}
		})
	}
}

// TestDefaultLocalEnvMatchesPattern verifies the default filename is covered by
// the local-file ignore pattern.
func TestDefaultLocalEnvMatchesPattern(t *testing.T) {
	t.Parallel()

	ws := loadLocalEnvPath(t, "")
	ok, err := filepath.Match(workspace.LocalFilePattern, filepath.Base(ws.LocalEnvPath))
	if err != nil || !ok {
		t.Errorf("%q does not match %q", ws.LocalEnvPath, workspace.LocalFilePattern)
	}
}
