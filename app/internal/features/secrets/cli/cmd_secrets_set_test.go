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

// writeSetManifest creates a valid workspace manifest for set action tests.
func writeSetManifest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestExecuteSetEncryptsAndStoresSafeMetadata verifies the action stores an
// encrypted envelope without persisting the supplied plaintext.
func TestExecuteSetEncryptsAndStoresSafeMetadata(t *testing.T) {
	t.Parallel()

	manifest := writeSetManifest(t)
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
	readSecret := func() (string, error) {
		return newSecretPrompt(secretPromptParams{
			Stdin:  strings.NewReader(plaintext + "\n"),
			Stderr: new(bytes.Buffer),
		}).readSecret()
	}
	result, err := manager.SetSecret(
		"Production", "database_password", readSecret,
	)
	if err != nil {
		t.Fatalf("SetSecret(): %v", err)
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

	data, err := filex.Read(resolved.Secrets.SecretsPath)
	if err != nil {
		t.Fatalf("Read(): %v", err)
	}
	if !strings.Contains(string(data), "encrypted-age:") {
		t.Errorf("store = %q, want an age ciphertext envelope", data)
	}
	if strings.Contains(string(data), plaintext) {
		t.Fatalf("store contains plaintext: %q", data)
	}
}

// TestExecuteSetRejectsUnconfirmedTerminalInputWithoutMutation verifies a failed
// confirmation cannot encrypt or write the secrets store.
func TestExecuteSetRejectsUnconfirmedTerminalInputWithoutMutation(t *testing.T) {
	t.Parallel()

	manifest := writeSetManifest(t)
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
	before, err := filex.Read(resolved.Secrets.SecretsPath)
	if err != nil {
		t.Fatalf("Read() before execute: %v", err)
	}

	terminal, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("Open(%q): %v", os.DevNull, err)
	}
	defer func() { _ = terminal.Close() }()
	var stderr bytes.Buffer
	readSecret := func() (string, error) {
		return newSecretPrompt(secretPromptParams{
			Stdin:  terminal,
			Stderr: &stderr,
			IsTerminal: func(int) bool {
				return true
			},
			ReadPassword: func(int) ([]byte, error) {
				return []byte("secret"), nil
			},
			ReadConfirmation: func(*os.File) (bool, error) {
				return false, nil
			},
		}).readSecret()
	}
	_, err = manager.SetSecret("production", "database_password", readSecret)
	if err == nil || err.Error() != "secret was not confirmed" {
		t.Fatalf("SetSecret() error = %v, want mismatch error", err)
	}
	after, err := filex.Read(resolved.Secrets.SecretsPath)
	if err != nil {
		t.Fatalf("Read() after execute: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf(
			"secrets store changed after mismatch:\nbefore: %s\nafter: %s",
			before,
			after,
		)
	}
	if stderr.String() != "Secret value: \nConfirm secret of length 6? [Y/n] " {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// TestOutputSetReportsKeyGroupAndStorePath verifies the safe mutation summary
// logs the stored identity and store path without leaking plaintext.
func TestOutputSetReportsKeyGroupAndStorePath(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputSecretsSet(
		printer.NewPlain(&out, &errOut),
		secrets.SetSecretResult{
			Location: "/workspace/secrets.yaml",
			Secret: secrets.SecretReference{
				Group: "production",
				Key:   "api_key",
			},
		},
	)
	if err != nil {
		t.Fatalf("outputSet(): %v", err)
	}

	got := out.String()
	for _, want := range []string{`"api_key"`, `"production"`, "/workspace/secrets.yaml"} {
		if !strings.Contains(got, want) {
			t.Errorf("outputSet() = %q, want it to contain %q", got, want)
		}
	}
}

// TestNewSecretsSetCommand verifies the command builds and dispatches through
// the factory.
func TestNewSecretsSetCommand(t *testing.T) {
	manifest := writeSetManifest(t)
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

	factory := &mockSecretsFactory{svc: manager}

	parent := NewSecretsCommand(factory)
	parent.PersistentFlags().String("config", manifest, "")
	parent.SetArgs([]string{
		"set",
		"production",
		"database_password",
		"my-secret-value",
		"--config", manifest,
	})

	var out bytes.Buffer
	parent.SetOut(&out)

	if err := parent.Execute(); err != nil {
		t.Fatalf("parent.Execute(): %v", err)
	}
}
