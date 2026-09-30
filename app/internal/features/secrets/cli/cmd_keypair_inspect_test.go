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

// writeKeypairInspectManifest creates the smallest valid workspace for
// management commands.
func writeKeypairInspectManifest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestInspectAndRender verifies valid and unavailable statuses without exposing
// private-key material.
func TestInspectAndRender(t *testing.T) {
	manifest := writeKeypairInspectManifest(t)
	in := &core.Input{ConfigPath: &manifest}
	resolved, err := core.ResolveWorkspace(in)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	secretManager, err := core.NewSecretsManager(
		resolved.Secrets,
		resolved.Cipher,
	)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	if _, err := secretManager.GenerateKeypair("production"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}

	metadata, err := secretManager.InspectKeypair("Production")
	if err != nil {
		t.Fatalf("InspectKeypair(): %v", err)
	}
	if metadata.PrivateKeyStatus != "valid" {
		t.Errorf("PrivateKeyStatus = %q, want valid", metadata.PrivateKeyStatus)
	}

	privateData, err := filex.Read(resolved.Secrets.KeysPath)
	if err != nil {
		t.Fatalf("read private-key file: %v", err)
	}
	if len(privateData) == 0 {
		t.Fatal("private-key file is empty")
	}

	if err := os.Remove(resolved.Secrets.KeysPath); err != nil {
		t.Fatalf("remove private-key file: %v", err)
	}
	metadata, err = secretManager.InspectKeypair("production")
	if err != nil {
		t.Fatalf("InspectKeypair() without key: %v", err)
	}
	if metadata.PrivateKeyStatus != "not_available" {
		t.Errorf(
			"missing-key status = %q, want not_available",
			metadata.PrivateKeyStatus,
		)
	}
}

// TestNewKeypairInspectCommand verifies inspection dispatches through the factory.
func TestNewKeypairInspectCommand(t *testing.T) {
	manifest := writeKeypairInspectManifest(t)
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

	parent := NewKeypairCmd(&mockSecretsFactory{svc: manager})
	parent.PersistentFlags().String("config", manifest, "")
	parent.SetArgs([]string{"inspect", "production", "--config", manifest})

	var out bytes.Buffer
	parent.SetOut(&out)
	if err := parent.Execute(); err != nil {
		t.Fatalf("parent.Execute(): %v", err)
	}
	if !strings.Contains(out.String(), "Keypair for group") {
		t.Errorf("parent.Execute() output = %q, want inspection summary", out.String())
	}
}

// TestOutputInspectDisplaysStatus verifies the status is formatted.
func TestOutputInspectDisplaysStatus(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	err := outputKeypairInspect(
		printer.NewPlain(&out, &errOut),
		secrets.KeypairMetadata{
			Group:            "production",
			PublicKey:        "age1ql3z7hjy...",
			PrivateKeyStatus: "valid",
		},
	)
	if err != nil {
		t.Fatalf("outputInspect(): %v", err)
	}

	got := out.String()
	for _, want := range []string{"production", "age1ql3z7hjy...", "valid"} {
		if !strings.Contains(got, want) {
			t.Errorf("outputInspect() = %q, want it to contain %q", got, want)
		}
	}
}
