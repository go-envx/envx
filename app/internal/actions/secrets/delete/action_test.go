package delete

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/pkg/file"
)

// writeManifest creates a valid workspace manifest for delete action tests.
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

// seedSecret generates a group keypair and stores one secret for delete tests.
func seedSecret(t *testing.T, input *config.Input, group, key, plaintext string) {
	t.Helper()
	resolved, err := config.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := config.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	if _, err := manager.GenerateKeypair(group); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if err := manager.Set(group, key, func() (string, error) {
		return plaintext, nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}
}

// TestExecuteRemovesStoredSecret verifies delete removes the value from the
// store while preserving the group's public key.
func TestExecuteRemovesStoredSecret(t *testing.T) {
	t.Parallel()

	manifest := writeManifest(t)
	input := &config.Input{ConfigPath: &manifest}
	seedSecret(t, input, "production", "database_password", "database-password")

	result, err := execute(
		actionParams{Group: "Production", Key: "database_password"},
		input,
	)
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if result.Group != "production" || result.Key != "database_password" {
		t.Errorf("result = %+v", result)
	}

	resolved, err := config.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := config.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	exists, err := manager.Has("production", "database_password")
	if err != nil {
		t.Fatalf("Has(): %v", err)
	}
	if exists {
		t.Error("execute() left the removed secret in the store")
	}

	data, err := file.Read(resolved.Secrets.SecretsPath)
	if err != nil {
		t.Fatalf("Read(): %v", err)
	}
	if !strings.Contains(string(data), "public_keys") {
		t.Errorf("store = %q, want the group's public key preserved", data)
	}
}

// TestExecuteMissingSecretFails verifies deleting a missing secret is an error.
func TestExecuteMissingSecretFails(t *testing.T) {
	t.Parallel()

	manifest := writeManifest(t)
	input := &config.Input{ConfigPath: &manifest}
	seedSecret(t, input, "production", "database_password", "database-password")

	if _, err := execute(
		actionParams{Group: "production", Key: "missing"},
		input,
	); err == nil {
		t.Fatal("execute() succeeded for a missing secret")
	}
}
