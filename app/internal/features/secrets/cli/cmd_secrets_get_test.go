package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/core"
)

// writeGetManifest creates a valid workspace manifest for get action tests.
func writeGetManifest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// seedGetSecret generates a group keypair and stores one secret for get tests.
func seedGetSecret(
	t *testing.T,
	input *core.Input,
	group, key, plaintext string,
) {
	t.Helper()
	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	if _, err := manager.GenerateKeypair(group); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if _, err := manager.SetSecret(group, key, func() (string, error) {
		return plaintext, nil
	}); err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}
}

// TestServiceGetDecryptsStoredSecret verifies service get returns the
// decrypted plaintext.
func TestServiceGetDecryptsStoredSecret(t *testing.T) {
	t.Parallel()

	manifest := writeGetManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	const plaintext = "database-password"
	seedGetSecret(t, input, "production", "database_password", plaintext)

	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}

	result, err := manager.GetSecret("Production", "database_password")
	if err != nil {
		t.Fatalf("GetSecret(): %v", err)
	}
	if result.Value != plaintext {
		t.Errorf("GetSecret() value = %q, want %q", result.Value, plaintext)
	}
	if result.Location != resolved.Secrets.SecretsPath {
		t.Errorf(
			"GetSecret() location = %q, want %q",
			result.Location,
			resolved.Secrets.SecretsPath,
		)
	}
}

// TestServiceGetMissingSecretFails verifies reading a missing secret is an error.
func TestServiceGetMissingSecretFails(t *testing.T) {
	t.Parallel()

	manifest := writeGetManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	seedGetSecret(t, input, "production", "database_password", "database-password")

	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}

	if _, err := manager.GetSecret("production", "missing"); err == nil {
		t.Fatal("GetSecret() succeeded for a missing secret")
	}
}

// TestOutputGetPrintsPlaintextWithNewline verifies the decrypted value is printed
// directly to the writer followed by a trailing newline.
func TestOutputGetPrintsPlaintextWithNewline(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := outputSecretsGet(&buf, "super-secret-password")
	if err != nil {
		t.Fatalf("outputGet(): %v", err)
	}

	got := buf.String()
	want := "super-secret-password\n"
	if got != want {
		t.Errorf("outputGet() = %q, want %q", got, want)
	}
}

// TestNewSecretsGetCommand verifies the command builds and dispatches through
// the factory.
func TestNewSecretsGetCommand(t *testing.T) {
	manifest := writeGetManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	const plaintext = "database-password"
	seedGetSecret(t, input, "production", "database_password", plaintext)

	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}

	factory := &mockSecretsFactory{svc: manager}

	parent := NewSecretsCommand(factory)
	parent.PersistentFlags().String("config", manifest, "")
	parent.SetArgs([]string{
		"get",
		"production",
		"database_password",
		"--config", manifest,
	})

	var out bytes.Buffer
	parent.SetOut(&out)

	if err := parent.Execute(); err != nil {
		t.Fatalf("parent.Execute(): %v", err)
	}

	if out.String() != plaintext+"\n" {
		t.Errorf("parent.Execute() output = %q, want %q", out.String(), plaintext+"\n")
	}
}
