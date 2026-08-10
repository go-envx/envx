package set

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/pkg/file"
)

// writeManifest creates a valid workspace manifest for set action tests.
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

// TestExecuteEncryptsAndStoresSafeMetadata verifies the action stores an
// encrypted envelope without persisting the supplied plaintext.
func TestExecuteEncryptsAndStoresSafeMetadata(t *testing.T) {
	t.Parallel()

	manifest := writeManifest(t)
	input := &config.Input{ConfigPath: &manifest}
	resolved, err := config.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := config.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	if _, err := manager.GenerateKeypair("production"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}

	const plaintext = "database-password"
	result, err := execute(
		actionParams{Group: "Production", Key: "database_password"},
		input,
		readerParams{
			Stdin:  strings.NewReader(plaintext + "\n"),
			Stderr: new(bytes.Buffer),
		},
	)
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if result.Group != "production" || result.Key != "database_password" {
		t.Errorf("result = %+v", result)
	}

	data, err := file.Read(resolved.Secrets.SecretsPath)
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

// TestPlaintextSourceUsesExplicitValue verifies an explicit empty value is
// preserved and stdin is not consulted when the argument is present.
func TestPlaintextSourceUsesExplicitValue(t *testing.T) {
	t.Parallel()

	plaintext := "argument-value"
	stdin := strings.NewReader("stdin-value\n")
	got, err := plaintextSource(
		actionParams{Plaintext: &plaintext},
		readerParams{Stdin: stdin},
	)
	if err != nil {
		t.Fatalf("plaintextSource(): %v", err)
	}
	if got != plaintext {
		t.Errorf("plaintextSource() = %q, want %q", got, plaintext)
	}
	if stdin.Len() != len("stdin-value\n") {
		t.Error("plaintextSource() consumed stdin for an explicit value")
	}

	empty := ""
	got, err = plaintextSource(
		actionParams{Plaintext: &empty},
		readerParams{Stdin: strings.NewReader("stdin-value\n")},
	)
	if err != nil {
		t.Fatalf("plaintextSource() with empty argument: %v", err)
	}
	if got != "" {
		t.Errorf("plaintextSource() with empty argument = %q, want empty", got)
	}
}

// TestExecuteRejectsUnconfirmedTerminalInputWithoutMutation verifies a failed
// confirmation cannot encrypt or write the secrets store.
func TestExecuteRejectsUnconfirmedTerminalInputWithoutMutation(t *testing.T) {
	t.Parallel()

	manifest := writeManifest(t)
	input := &config.Input{ConfigPath: &manifest}
	resolved, err := config.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := config.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	if _, err := manager.GenerateKeypair("production"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	before, err := file.Read(resolved.Secrets.SecretsPath)
	if err != nil {
		t.Fatalf("Read() before execute: %v", err)
	}

	terminal, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("Open(%q): %v", os.DevNull, err)
	}
	defer func() { _ = terminal.Close() }()
	var stderr bytes.Buffer
	_, err = execute(
		actionParams{Group: "production", Key: "database_password"},
		input,
		readerParams{
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
		},
	)
	if err == nil || err.Error() != "secret was not confirmed" {
		t.Fatalf("execute() error = %v, want mismatch error", err)
	}
	after, err := file.Read(resolved.Secrets.SecretsPath)
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
