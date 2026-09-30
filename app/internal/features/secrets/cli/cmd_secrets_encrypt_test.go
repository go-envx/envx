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

// writeEncryptManifest creates a valid workspace manifest for encrypt action tests.
func writeEncryptManifest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestExecuteEncryptsPlaintextInPlace verifies encrypt replaces plaintext with
// ciphertext envelopes without altering untouched groups.
func TestExecuteEncryptsPlaintextInPlace(t *testing.T) {
	t.Parallel()

	manifest := writeEncryptManifest(t)
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
	if _, err := manager.SetSecret("production", "api_key", func() (string, error) {
		return "plain-secret", nil
	}); err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	// First decrypt it so we have a plaintext store value
	if _, err := manager.DecryptSecrets("production", "api_key"); err != nil {
		t.Fatalf("DecryptSecrets(): %v", err)
	}

	result, err := manager.EncryptSecrets("Production", "api_key")
	if err != nil {
		t.Fatalf("EncryptSecrets(): %v", err)
	}
	if len(result.Secrets) != 1 {
		t.Fatalf("len(Secrets) = %d, want 1", len(result.Secrets))
	}
	if result.Secrets[0].Group != "production" || result.Secrets[0].Key != "api_key" {
		t.Errorf("Secrets = %+v", result.Secrets)
	}

	data, err := filex.Read(resolved.Secrets.SecretsPath)
	if err != nil {
		t.Fatalf("Read(): %v", err)
	}
	if !strings.Contains(string(data), "encrypted-age:") {
		t.Errorf("store = %q, want age envelope", data)
	}
}

// TestExecuteEncryptNoMatchesFails verifies narrowing to an unknown group fails.
func TestExecuteEncryptNoMatchesFails(t *testing.T) {
	t.Parallel()

	manifest := writeEncryptManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}

	if _, err := manager.EncryptSecrets("missing", ""); err == nil {
		t.Fatal("EncryptSecrets() succeeded for missing group")
	}
}

// TestOutputEncryptReportsEncryptedCount verifies the summary output.
func TestOutputEncryptReportsEncryptedCount(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputSecretsEncrypt(
		printer.NewPlain(&out, &errOut),
		secrets.EncryptSecretsResult{
			Secrets: []secrets.SecretReference{
				{Group: "production", Key: "api_key"},
			},
			Location: "/workspace/secrets.yaml",
		},
		true,
	)
	if err != nil {
		t.Fatalf("outputEncrypt(): %v", err)
	}

	got := out.String()
	wants := []string{
		"Encrypted 1 secret",
		"/workspace/secrets.yaml",
		"production/api_key",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("outputEncrypt() = %q, want it to contain %q", got, want)
		}
	}
}

// TestOutputEncryptReportsNothingToEncrypt verifies message when no changes.
func TestOutputEncryptReportsNothingToEncrypt(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputSecretsEncrypt(
		printer.NewPlain(&out, &errOut),
		secrets.EncryptSecretsResult{
			Secrets:  nil,
			Location: "/workspace/secrets.yaml",
		},
		false,
	)
	if err != nil {
		t.Fatalf("outputEncrypt(): %v", err)
	}

	if !strings.Contains(out.String(), "No plaintext values to encrypt.") {
		t.Errorf("outputEncrypt() = %q, want no plaintext message", out.String())
	}
}

// TestNewSecretsEncryptCommand verifies the command builds and dispatches
// through the factory.
func TestNewSecretsEncryptCommand(t *testing.T) {
	manifest := writeEncryptManifest(t)
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
	if _, err := manager.SetSecret("production", "api_key", func() (string, error) {
		return "plain-secret", nil
	}); err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	// Decrypt so we have a plaintext secret to encrypt
	if _, err := manager.DecryptSecrets("production", "api_key"); err != nil {
		t.Fatalf("DecryptSecrets(): %v", err)
	}

	factory := &mockSecretsFactory{svc: manager}

	parent := NewSecretsCommand(factory)
	parent.PersistentFlags().String("config", manifest, "")
	parent.SetArgs([]string{
		"encrypt",
		"--group", "production",
		"--key", "api_key",
		"--config", manifest,
	})

	var out bytes.Buffer
	parent.SetOut(&out)

	if err := parent.Execute(); err != nil {
		t.Fatalf("parent.Execute(): %v", err)
	}
}
