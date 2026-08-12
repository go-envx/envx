package get

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
)

// writeManifest creates a valid workspace manifest for get action tests.
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

// seedSecret generates a group keypair and stores one secret for get tests.
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

// TestExecuteDecryptsStoredSecret verifies get returns the decrypted plaintext.
func TestExecuteDecryptsStoredSecret(t *testing.T) {
	t.Parallel()

	manifest := writeManifest(t)
	input := &config.Input{ConfigPath: &manifest}
	const plaintext = "database-password"
	seedSecret(t, input, "production", "database_password", plaintext)

	result, err := execute(
		actionParams{Group: "Production", Key: "database_password"},
		input,
	)
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if result.Value != plaintext {
		t.Errorf("execute() value = %q, want %q", result.Value, plaintext)
	}
}

// TestExecuteMissingSecretFails verifies reading a missing secret is an error.
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
