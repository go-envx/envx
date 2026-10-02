package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/resources/cipher"
)

// TestNewSecretsServiceUsesConfiguredAlgorithm verifies service composition
// passes the selected cipher into the root secrets workflow.
func TestNewSecretsServiceUsesConfiguredAlgorithm(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewSecretsService(
		filepath.Join(dir, "secrets.yaml"),
		filepath.Join(dir, "envx.keys"),
		cipher.Params{Algorithm: cipher.NaClBox},
		2,
	)
	if err != nil {
		t.Fatalf("NewSecretsService(): %v", err)
	}
	result, err := manager.GenerateKeypair("production")
	if err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if !strings.HasPrefix(result.Keypair.PublicKey, "nacl-box-public-key:") {
		t.Errorf("PublicKey = %q, want NaCl Box key", result.Keypair.PublicKey)
	}
	if _, err := os.Stat(filepath.Join(dir, "envx.keys")); err != nil {
		t.Fatalf("private-key file was not written: %v", err)
	}
}
