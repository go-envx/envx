package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/pkg/file"
)

// writeManifest creates the smallest valid workspace for management commands.
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

// TestExecuteAndRender verifies generation writes through the manager and does
// not render private-key bytes.
func TestExecuteAndRender(t *testing.T) {
	manifest := writeManifest(t)
	result, err := execute(
		actionParams{Group: "Production"},
		&config.Input{ConfigPath: &manifest},
	)
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if result.Metadata.Group != "production" {
		t.Errorf("Group = %q, want production", result.Metadata.Group)
	}
	if result.Metadata.PrivateKeyStatus != "valid" {
		t.Errorf("PrivateKeyStatus = %q, want valid", result.Metadata.PrivateKeyStatus)
	}

	privateData, err := file.Read(result.KeysPath)
	if err != nil {
		t.Fatalf("read private-key file: %v", err)
	}
	if len(privateData) == 0 {
		t.Fatal("private-key file is empty")
	}
}

// TestExecuteUsesConfiguredCipher verifies normal generation uses the manifest
// algorithm before persisting the keypair through the manager.
func TestExecuteUsesConfiguredCipher(t *testing.T) {
	t.Parallel()

	manifest := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"secrets:\n  cipher: nacl-box\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(manifest, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := execute(
		actionParams{Group: "production"},
		&config.Input{ConfigPath: &manifest},
	)
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if !strings.HasPrefix(result.Metadata.PublicKey, "nacl-box-public-key:") {
		t.Errorf("public key = %q, want NaCl Box key", result.Metadata.PublicKey)
	}
	privateData, err := file.Read(result.KeysPath)
	if err != nil {
		t.Fatalf("read private-key file: %v", err)
	}
	if !strings.Contains(string(privateData), "nacl-box-private-key:") {
		t.Errorf("private-key file = %q, want NaCl Box key", privateData)
	}
}
