package print

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/pkg/file"
)

// -------------------------------------------------------------------------------------

// TestExecuteUsesConfiguredCipher verifies print uses the manifest algorithm
// and does not change managed workspace files.
func TestExecuteUsesConfiguredCipher(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := filepath.Join(dir, "envx.yaml")
	body := "environments: [production]\n" +
		"secrets:\n  cipher: nacl-box\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(manifest, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	secretsPath := filepath.Join(dir, "secrets.yaml")
	keysPath := filepath.Join(dir, "envx.keys")
	if err := os.WriteFile(secretsPath, []byte("existing secrets\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keysPath, []byte("existing keys\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeSecrets, err := file.Read(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeKeys, err := file.Read(keysPath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := execute(&config.Input{ConfigPath: &manifest}, "")
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if !strings.HasPrefix(result.Keypair.PublicKey, "nacl-box-public-key:") {
		t.Errorf("public key = %q, want NaCl Box key", result.Keypair.PublicKey)
	}
	if !strings.HasPrefix(result.Keypair.PrivateKey, "nacl-box-private-key:") {
		t.Errorf("private key = %q, want NaCl Box key", result.Keypair.PrivateKey)
	}

	afterSecrets, err := file.Read(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	afterKeys, err := file.Read(keysPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeSecrets, afterSecrets) || !bytes.Equal(beforeKeys, afterKeys) {
		t.Fatal("print changed existing workspace files")
	}
}

// -------------------------------------------------------------------------------------

// TestExecuteUsesEmptyCipherAsFallback verifies an explicit empty cipher value
// follows the same manifest fallback as an omitted value.
func TestExecuteUsesEmptyCipherAsFallback(t *testing.T) {
	t.Parallel()

	manifest := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"secrets:\n  cipher: nacl-box\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(manifest, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := execute(&config.Input{ConfigPath: &manifest}, "")
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if !strings.HasPrefix(result.Keypair.PublicKey, "nacl-box-public-key:") {
		t.Errorf(
			"public key = %q, want manifest-selected NaCl Box key",
			result.Keypair.PublicKey,
		)
	}
}

// -------------------------------------------------------------------------------------

// TestExecuteUsesExplicitCipher verifies an explicit cipher skips manifest
// lookup and constructs the requested implementation directly.
func TestExecuteUsesExplicitCipher(t *testing.T) {
	t.Parallel()

	missingManifest := filepath.Join(t.TempDir(), "missing.yaml")
	cipherName := string(cipher.NaClBox)
	result, err := execute(
		&config.Input{ConfigPath: &missingManifest},
		cipherName,
	)
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if !strings.HasPrefix(result.Keypair.PublicKey, "nacl-box-public-key:") {
		t.Errorf("public key = %q, want NaCl Box key", result.Keypair.PublicKey)
	}
}

// -------------------------------------------------------------------------------------

// TestExecuteUsesDefaultCipherWithoutManifest verifies the fallback remains
// Age when no explicit cipher or workspace configuration is available.
func TestExecuteUsesDefaultCipherWithoutManifest(t *testing.T) {
	t.Parallel()

	missingManifest := filepath.Join(t.TempDir(), "missing.yaml")
	result, err := execute(
		&config.Input{ConfigPath: &missingManifest},
		"",
	)
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if !strings.HasPrefix(result.Keypair.PublicKey, "age-public-key:") {
		t.Errorf("public key = %q, want Age key", result.Keypair.PublicKey)
	}
}
