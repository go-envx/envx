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

// writeDecryptManifest creates a valid workspace manifest for decrypt action tests.
func writeDecryptManifest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestExecuteDecryptsCiphertextInPlace verifies decrypt rewrites ciphertext with
// plaintext in place.
func TestExecuteDecryptsCiphertextInPlace(t *testing.T) {
	t.Parallel()

	manifest := writeDecryptManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	if _, err := manager.GenerateKeypair("production"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}

	const plaintext = "database-password"
	_, err = manager.SetSecret(
		"production", "database_password", func() (string, error) {
			return plaintext, nil
		},
	)
	if err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	result, err := manager.DecryptSecrets("Production", "database_password")
	if err != nil {
		t.Fatalf("DecryptSecrets(): %v", err)
	}
	if len(result.Secrets) != 1 {
		t.Fatalf("len(Secrets) = %d, want 1", len(result.Secrets))
	}

	data, err := filex.Read(resolved.Secrets.SecretsPath)
	if err != nil {
		t.Fatalf("Read(): %v", err)
	}
	if !strings.Contains(string(data), plaintext) {
		t.Fatalf("store does not contain plaintext after decrypt: %q", data)
	}
}

// TestExecuteDecryptUnavailableGroupReportsSkipped verifies missing key reports
// in unavailable.
func TestExecuteDecryptUnavailableGroupReportsSkipped(t *testing.T) {
	t.Parallel()

	manifest := writeDecryptManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	if _, err := manager.GenerateKeypair("production"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	_, err = manager.SetSecret(
		"production", "database_password", func() (string, error) {
			return "secret-val", nil
		},
	)
	if err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	// Remove keys file so group is unavailable
	if err := os.Remove(resolved.Secrets.KeysPath); err != nil {
		t.Fatalf("Remove keys file: %v", err)
	}

	result, err := manager.DecryptSecrets("", "")
	if err != nil {
		t.Fatalf("DecryptSecrets(): %v", err)
	}
	if len(result.UnavailableGroups) != 1 || result.UnavailableGroups[0] != "production" {
		t.Errorf("result.UnavailableGroups = %v, want [production]", result.UnavailableGroups)
	}
}

// TestOutputDecryptReportsDecryptedCount verifies the summary output.
func TestOutputDecryptReportsDecryptedCount(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputSecretsDecrypt(
		printer.NewPlain(&out, &errOut),
		secrets.DecryptSecretsResult{
			Secrets: []secrets.SecretReference{
				{Group: "production", Key: "database_password"},
			},
			Location: "/workspace/secrets.yaml",
		},
		true,
	)
	if err != nil {
		t.Fatalf("outputDecrypt(): %v", err)
	}

	got := out.String()
	wants := []string{
		"Decrypted 1 secret",
		"/workspace/secrets.yaml",
		"production/database_password",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("outputDecrypt() = %q, want it to contain %q", got, want)
		}
	}
}

// mockSecretsFactory implements SecretsFactory for unit testing.
type mockSecretsFactory struct {
	svc *secrets.Service
	err error
}

func (m *mockSecretsFactory) SecretsService(
	_ string,
) (*secrets.Service, error) {
	return m.svc, m.err
}

// TestNewDecryptCommand verifies the command builds and dispatches through the factory.
func TestNewDecryptCommand(t *testing.T) {
	manifest := writeDecryptManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	if _, err := manager.GenerateKeypair("production"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	_, err = manager.SetSecret(
		"production", "database_password", func() (string, error) {
			return "secret-value", nil
		},
	)
	if err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	factory := &mockSecretsFactory{svc: manager}

	parent := NewSecretsCommand(factory)
	parent.PersistentFlags().String("config", manifest, "")
	parent.SetArgs([]string{
		"decrypt",
		"--group", "production",
		"--key", "database_password",
		"--config", manifest,
	})

	var out bytes.Buffer
	parent.SetOut(&out)

	if err := parent.Execute(); err != nil {
		t.Fatalf("parent.Execute(): %v", err)
	}
}
