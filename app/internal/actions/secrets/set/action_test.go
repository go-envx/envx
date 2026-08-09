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

// -------------------------------------------------------------------------------------

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

// -------------------------------------------------------------------------------------

// TestExecuteEncryptsAndRendersSafeMetadata verifies the action stores an
// encrypted envelope and never renders the supplied plaintext.
func TestExecuteEncryptsAndRendersSafeMetadata(t *testing.T) {
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
		streams{
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

	var output bytes.Buffer
	if err := render(&renderParams{Writer: &output, Result: result}); err != nil {
		t.Fatalf("render(): %v", err)
	}
	if strings.Contains(output.String(), plaintext) {
		t.Fatalf("rendered output contains plaintext: %q", output.String())
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

// -------------------------------------------------------------------------------------

// TestReadPlaintext verifies stdin input removes only the shell's final line
// ending and rejects an empty value.
func TestReadPlaintext(t *testing.T) {
	t.Parallel()

	got, err := readPlaintext(strings.NewReader("value\r\n"))
	if err != nil {
		t.Fatalf("readPlaintext(): %v", err)
	}
	if got != "value" {
		t.Errorf("readPlaintext() = %q, want value", got)
	}
	if _, err := readPlaintext(strings.NewReader("\n")); err == nil {
		t.Fatal("readPlaintext() accepted an empty value")
	}
}

// -------------------------------------------------------------------------------------

// TestCommandArgsValidation verifies Cobra rejects incorrect positional counts.
func TestCommandArgsValidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "missing args", args: []string{"production"}},
		{name: "extra args", args: []string{"production", "key", "extra"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := NewCommand()
			if err := cmd.Args(cmd, test.args); err == nil {
				t.Fatal("command Args validation succeeded")
			}
		})
	}
}
