package decrypt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/secrets"
	"github.com/go-envx/envx/app/pkg/file"
)

// writeManifest creates a valid workspace manifest for decrypt action tests.
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

// seedSecret generates a group keypair and stores one encrypted secret so
// decrypt has something to decrypt. It returns the store path.
func seedSecret(
	t *testing.T, input *config.Input, group, key, plaintext string,
) string {
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
	return resolved.Secrets.SecretsPath
}

// TestExecuteDecryptsCiphertext verifies decrypt rewrites an encrypted value as
// plaintext and reports the changed identity.
func TestExecuteDecryptsCiphertext(t *testing.T) {
	t.Parallel()

	manifest := writeManifest(t)
	input := &config.Input{ConfigPath: &manifest}
	storePath := seedSecret(t, input, "production", "api_key", "plain-value")

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
	if !strings.Contains(string(data), "plain-value") {
		t.Errorf("store = %q, want the decrypted plaintext", data)
	}
	if strings.Contains(string(data), "encrypted-age:") {
		t.Error("execute() left an encrypted envelope in the store")
	}
}

// TestExecuteSelectorMatchingNothingFails verifies an explicit group that
// matches no stored value is an error.
func TestExecuteSelectorMatchingNothingFails(t *testing.T) {
	t.Parallel()

	manifest := writeManifest(t)
	input := &config.Input{ConfigPath: &manifest}
	seedSecret(t, input, "production", "api_key", "plain-value")

	if _, err := execute(actionParams{Group: "missing"}, input); err == nil {
		t.Fatal("execute() accepted a selector matching nothing")
	}
}

// TestExecuteSkipsUnavailableKey verifies a group whose private key is missing is
// reported as unavailable and left encrypted rather than failing the command.
func TestExecuteSkipsUnavailableKey(t *testing.T) {
	t.Parallel()

	manifest := writeManifest(t)
	input := &config.Input{ConfigPath: &manifest}
	storePath := seedSecret(t, input, "production", "api_key", "plain-value")

	// Remove the local private-key file so the group's key is unavailable.
	keysPath := filepath.Join(filepath.Dir(storePath), "envx.keys")
	if err := os.Remove(keysPath); err != nil {
		t.Fatalf("Remove(envx.keys): %v", err)
	}

	result, err := execute(actionParams{}, input)
	if err != nil {
		t.Fatalf("execute() failed on an unavailable key: %v", err)
	}
	if len(result.Changed) != 0 {
		t.Errorf("changed = %+v, want none", result.Changed)
	}
	if len(result.Unavailable) != 1 || result.Unavailable[0] != "production" {
		t.Errorf("unavailable = %v, want [production]", result.Unavailable)
	}

	data, err := file.Read(storePath)
	if err != nil {
		t.Fatalf("Read(): %v", err)
	}
	if !strings.Contains(string(data), "encrypted-age:") {
		t.Errorf("store = %q, want the value left encrypted", data)
	}
}
