package inspect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/pkg/file"
)

// -------------------------------------------------------------------------------------

// writeManifest creates the smallest valid workspace for management commands.
func writeManifest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// -------------------------------------------------------------------------------------

// TestExecuteAndRender verifies valid and unavailable statuses without exposing
// private-key material.
func TestExecuteAndRender(t *testing.T) {
	manifest := writeManifest(t)
	in := &config.Input{ConfigPath: &manifest}
	resolved, err := config.ResolveWorkspace(in)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	secretManager, err := config.NewSecretsManager(
		resolved.Secrets,
		resolved.Cipher,
	)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	if _, err := secretManager.GenerateKeypair("production"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}

	metadata, err := execute(actionParams{Group: "Production"}, in)
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if metadata.PrivateKeyStatus != "valid" {
		t.Errorf("PrivateKeyStatus = %q, want valid", metadata.PrivateKeyStatus)
	}

	privateData, err := file.Read(resolved.Secrets.KeysPath)
	if err != nil {
		t.Fatalf("read private-key file: %v", err)
	}
	if len(privateData) == 0 {
		t.Fatal("private-key file is empty")
	}

	if err := os.Remove(resolved.Secrets.KeysPath); err != nil {
		t.Fatalf("remove private-key file: %v", err)
	}
	metadata, err = execute(actionParams{Group: "production"}, in)
	if err != nil {
		t.Fatalf("execute() without key: %v", err)
	}
	if metadata.PrivateKeyStatus != "not_available" {
		t.Errorf("missing-key status = %q, want not_available", metadata.PrivateKeyStatus)
	}
}
