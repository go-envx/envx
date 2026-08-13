package encrypt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/secrets"
	"github.com/go-envx/envx/app/pkg/file"
)

// writeManifest creates a valid workspace manifest for encrypt action tests.
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

// seedPlaintext generates a group keypair and injects one plaintext value into
// the store so encrypt has something to encrypt. It returns the store path.
func seedPlaintext(t *testing.T, input *config.Input, group, key, value string) string {
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

	storePath := resolved.Secrets.SecretsPath
	data, err := file.Read(storePath)
	if err != nil {
		t.Fatalf("Read(): %v", err)
	}
	body := string(data) + fmt.Sprintf(
		"secrets:\n  %s:\n    %s: %s\n", group, key, value,
	)
	if err := os.WriteFile(storePath, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	return storePath
}

// TestExecuteEncryptsPlaintext verifies encrypt replaces a plaintext value with
// an algorithm-tagged envelope and reports the changed identity.
func TestExecuteEncryptsPlaintext(t *testing.T) {
	t.Parallel()

	manifest := writeManifest(t)
	input := &config.Input{ConfigPath: &manifest}
	storePath := seedPlaintext(t, input, "production", "api_key", "plain-value")

	result, err := execute(actionParams{}, input)
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if len(result.Changed) != 1 ||
		result.Changed[0] != (secrets.SecretReference{Group: "production", Key: "api_key"}) {
		t.Fatalf("changed = %+v, want production/api_key", result.Changed)
	}

	data, err := file.Read(storePath)
	if err != nil {
		t.Fatalf("Read(): %v", err)
	}
	if strings.Contains(string(data), "plain-value") {
		t.Error("execute() left plaintext in the store")
	}
	if !strings.Contains(string(data), "encrypted-age:") {
		t.Errorf("store = %q, want an encrypted envelope", data)
	}
}

// TestExecuteSelectorMatchingNothingFails verifies an explicit group that
// matches no stored value is an error.
func TestExecuteSelectorMatchingNothingFails(t *testing.T) {
	t.Parallel()

	manifest := writeManifest(t)
	input := &config.Input{ConfigPath: &manifest}
	seedPlaintext(t, input, "production", "api_key", "plain-value")

	if _, err := execute(actionParams{Group: "missing"}, input); err == nil {
		t.Fatal("execute() accepted a selector matching nothing")
	}
}
