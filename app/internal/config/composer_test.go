package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/secrets"
)

// -------------------------------------------------------------------------------------

// TestNewConfiguredCipherUsesConfiguredAlgorithm verifies the config composer
// selects the manifest cipher and retains the age default without a manifest.
func TestNewConfiguredCipherUsesConfiguredAlgorithm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configPath string
		prefix     string
	}{
		{
			name:       "age default",
			configPath: filepath.Join(t.TempDir(), "missing.yaml"),
			prefix:     "age-public-key:",
		},
		{
			name:       "nacl box manifest",
			configPath: writeCipherManifest(t, string(cipher.NaClBox)),
			prefix:     "nacl-box-public-key:",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			selected, err := NewConfiguredCipher(
				&Input{ConfigPath: &test.configPath},
			)
			if err != nil {
				t.Fatalf("NewConfiguredCipher(): %v", err)
			}
			pair, err := selected.Keypair()
			if err != nil {
				t.Fatalf("Keypair(): %v", err)
			}
			if !strings.HasPrefix(pair.PublicKey, test.prefix) {
				t.Errorf("PublicKey = %q, want prefix %q", pair.PublicKey, test.prefix)
			}
		})
	}
}

// -------------------------------------------------------------------------------------

// writeCipherManifest creates a minimal manifest selecting algorithm.
func writeCipherManifest(t *testing.T, algorithm string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"secrets:\n  cipher: " + algorithm + "\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// -------------------------------------------------------------------------------------

// TestNewSecretsManagerUsesConfiguredAlgorithm verifies manager composition
// passes the selected cipher into the root secrets workflow.
func TestNewSecretsManagerUsesConfiguredAlgorithm(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewSecretsManager(secrets.Params{
		SecretsPath: filepath.Join(dir, "secrets.yaml"),
		KeysPath:    filepath.Join(dir, "envx.keys"),
	}, cipher.Params{
		Algorithm: cipher.NaClBox,
	})
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	metadata, err := manager.GenerateKeypair("production")
	if err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if !strings.HasPrefix(metadata.PublicKey, "nacl-box-public-key:") {
		t.Errorf("PublicKey = %q, want NaCl Box key", metadata.PublicKey)
	}
	if _, err := os.Stat(filepath.Join(dir, "envx.keys")); err != nil {
		t.Fatalf("private-key file was not written: %v", err)
	}
}
