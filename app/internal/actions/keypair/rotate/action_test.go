package rotate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/secrets"
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

// managerFor builds a secrets manager for the resolved workspace of in.
func managerFor(t *testing.T, in *config.Input) *secrets.Manager {
	t.Helper()
	resolved, err := config.ResolveWorkspace(in)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := config.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	return manager
}

// TestExecuteRotatesGroup verifies rotation re-encrypts the group through the
// manager and reports safe metadata without private-key bytes.
func TestExecuteRotatesGroup(t *testing.T) {
	manifest := writeManifest(t)
	in := &config.Input{ConfigPath: &manifest}
	manager := managerFor(t, in)

	if _, err := manager.GenerateKeypair("production"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if err := manager.Set("production", "api_key", func() (string, error) {
		return "plain-api", nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}

	result, err := execute(actionParams{Group: "Production"}, in)
	if err != nil {
		t.Fatalf("execute(): %v", err)
	}
	if len(result.Result.Keypairs) != 1 {
		t.Fatalf("Keypairs = %v, want one rotated keypair", result.Result.Keypairs)
	}
	if result.Result.Keypairs[0].Group != "production" {
		t.Errorf("Group = %q, want production", result.Result.Keypairs[0].Group)
	}
	if len(result.Result.Secrets) != 1 {
		t.Errorf("Secrets = %v, want one re-encrypted identity", result.Result.Secrets)
	}

	// The rotated store must still decrypt through the manager.
	value, err := managerFor(t, in).Get("production", "api_key")
	if err != nil {
		t.Fatalf("Get() after rotation: %v", err)
	}
	if value != "plain-api" {
		t.Errorf("Get() = %q, want plain-api", value)
	}

	privateData, err := file.Read(result.KeysPath)
	if err != nil {
		t.Fatalf("read private-key file: %v", err)
	}
	if len(privateData) == 0 {
		t.Fatal("private-key file is empty")
	}
}

// TestExecuteFailsForMissingGroup verifies rotation of an unknown group errors.
func TestExecuteFailsForMissingGroup(t *testing.T) {
	manifest := writeManifest(t)

	_, err := execute(
		actionParams{Group: "production"},
		&config.Input{ConfigPath: &manifest},
	)
	if err == nil {
		t.Fatal("execute() succeeded for a missing group")
	}
	if !strings.Contains(err.Error(), "no public key") {
		t.Errorf("error = %q, want missing-identity guidance", err)
	}
}
