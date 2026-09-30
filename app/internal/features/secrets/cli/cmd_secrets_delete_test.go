package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/utils/filex"
	"github.com/go-envx/envx/app/internal/utils/printer"
)

// writeDeleteManifest creates a valid workspace manifest for delete action tests.
func writeDeleteManifest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// seedDeleteSecret generates a group keypair and stores one secret for delete tests.
func seedDeleteSecret(t *testing.T, input *core.Input, group, key, plaintext string) {
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

// TestExecuteDeleteRemovesStoredSecret verifies delete removes the value from the
// store while preserving the group's public key.
func TestExecuteDeleteRemovesStoredSecret(t *testing.T) {
	t.Parallel()

	manifest := writeDeleteManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	seedDeleteSecret(
		t,
		input,
		"production",
		"database_password",
		"database-password",
	)

	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}

	result, err := manager.DeleteSecret("Production", "database_password")
	if err != nil {
		t.Fatalf("DeleteSecret(): %v", err)
	}
	if result.Secret.Group != "production" || result.Secret.Key != "database_password" {
		t.Errorf("result = %+v", result)
	}
	if result.Location != resolved.Secrets.SecretsPath {
		t.Errorf(
			"result.Location = %q, want %q",
			result.Location,
			resolved.Secrets.SecretsPath,
		)
	}

	exists, err := manager.Has("production", "database_password")
	if err != nil {
		t.Fatalf("Has(): %v", err)
	}
	if exists {
		t.Error("DeleteSecret() left the removed secret in the store")
	}

	data, err := filex.Read(resolved.Secrets.SecretsPath)
	if err != nil {
		t.Fatalf("Read(): %v", err)
	}
	if !strings.Contains(string(data), "public-keys") {
		t.Errorf("store = %q, want the group's public key preserved", data)
	}
}

// TestExecuteDeleteMissingSecretFails verifies deleting a missing secret is an error.
func TestExecuteDeleteMissingSecretFails(t *testing.T) {
	t.Parallel()

	manifest := writeDeleteManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	seedDeleteSecret(
		t,
		input,
		"production",
		"database_password",
		"database-password",
	)

	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}

	if _, err := manager.DeleteSecret("production", "missing"); err == nil {
		t.Fatal("DeleteSecret() succeeded for a missing secret")
	}
}

// TestOutputDeleteReportsKeyGroupAndStorePath verifies the safe mutation summary
// logs the deleted identity and store path.
func TestOutputDeleteReportsKeyGroupAndStorePath(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputSecretsDelete(
		printer.NewPlain(&out, &errOut),
		secrets.DeleteSecretResult{
			Location: "/workspace/secrets.yaml",
			Secret: secrets.SecretReference{
				Group: "production",
				Key:   "database_password",
			},
		},
	)
	if err != nil {
		t.Fatalf("outputDelete(): %v", err)
	}

	got := out.String()
	wants := []string{
		`"database_password"`,
		`"production"`,
		"/workspace/secrets.yaml",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("outputDelete() = %q, want it to contain %q", got, want)
		}
	}
}

// TestNewSecretsDeleteCommand verifies the command builds and dispatches
// through the factory.
func TestNewSecretsDeleteCommand(t *testing.T) {
	manifest := writeDeleteManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	seedDeleteSecret(
		t,
		input,
		"production",
		"database_password",
		"database-password",
	)

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
		"delete",
		"production",
		"database_password",
		"--config", manifest,
	})

	var out bytes.Buffer
	parent.SetOut(&out)

	if err := parent.Execute(); err != nil {
		t.Fatalf("parent.Execute(): %v", err)
	}
}
