package secrets_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/features/workspace"
)

// TestLoadConfig verifies the secrets config mirrors the normalized workspace.
func TestLoadConfig(t *testing.T) {
	t.Parallel()

	ws := &workspace.Workspace{
		Indent: 4,
		Secrets: workspace.SecretsConfig{
			SecretsPath: "/work/secrets.yaml",
			KeysPath:    "/work/envx.keys",
			Cipher:      "nacl-box",
		},
	}

	got, err := secrets.LoadConfig(ws)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := secrets.Config{
		SecretsPath:   "/work/secrets.yaml",
		KeysPath:      "/work/envx.keys",
		Cipher:        "nacl-box",
		DefaultIndent: 4,
	}
	if got != want {
		t.Errorf("LoadConfig() = %+v, want %+v", got, want)
	}
}

// TestLoadConfigRejectsNilWorkspace verifies a missing workspace is an error.
func TestLoadConfigRejectsNilWorkspace(t *testing.T) {
	t.Parallel()

	if _, err := secrets.LoadConfig(nil); err == nil {
		t.Fatal("LoadConfig(nil) succeeded")
	}
}
