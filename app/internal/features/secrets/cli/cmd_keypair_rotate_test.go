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

// writeKeypairRotateManifest creates the smallest valid workspace for
// management commands.
func writeKeypairRotateManifest(t *testing.T) string {
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
func managerFor(t *testing.T, in *core.Input) *secrets.Service {
	t.Helper()
	resolved, err := core.ResolveWorkspace(in)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	return manager
}

// TestRotateGroup verifies rotation re-encrypts the group through the
// manager and reports safe metadata without private-key bytes.
func TestRotateGroup(t *testing.T) {
	manifest := writeKeypairRotateManifest(t)
	in := &core.Input{ConfigPath: &manifest}
	manager := managerFor(t, in)
	resolved, err := core.ResolveWorkspace(in)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}

	if _, err := manager.GenerateKeypair("production"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if _, err := manager.SetSecret("production", "api_key", func() (string, error) {
		return "plain-api", nil
	}); err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	result, err := manager.RotateKeypair("Production")
	if err != nil {
		t.Fatalf("RotateKeypair(): %v", err)
	}
	if result.Keypair.Group != "production" {
		t.Errorf(
			"Group = %q, want production",
			result.Keypair.Group,
		)
	}
	if len(result.Secrets) != 1 {
		t.Errorf(
			"Secrets = %v, want one re-encrypted identity",
			result.Secrets,
		)
	}

	// The rotated store must still decrypt through the manager.
	res, err := managerFor(t, in).GetSecret("production", "api_key")
	if err != nil {
		t.Fatalf("GetSecret() after rotation: %v", err)
	}
	if res.Value != "plain-api" {
		t.Errorf("GetSecret() = %q, want plain-api", res.Value)
	}

	privateData, err := filex.Read(resolved.Secrets.KeysPath)
	if err != nil {
		t.Fatalf("read private-key file: %v", err)
	}
	if len(privateData) == 0 {
		t.Fatal("private-key file is empty")
	}
}

// TestRotateFailsForMissingGroup verifies rotation of an unknown group errors.
func TestRotateFailsForMissingGroup(t *testing.T) {
	manifest := writeKeypairRotateManifest(t)

	in := &core.Input{ConfigPath: &manifest}
	manager := managerFor(t, in)
	_, err := manager.RotateKeypair("production")
	if err == nil {
		t.Fatal("RotateKeypair() succeeded for a missing group")
	}
	if !strings.Contains(err.Error(), "no public key") {
		t.Errorf("error = %q, want missing-identity guidance", err)
	}
}

// TestNewKeypairRotateCommand verifies rotation dispatches through the factory.
func TestNewKeypairRotateCommand(t *testing.T) {
	manifest := writeKeypairRotateManifest(t)
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
	parent.SetArgs([]string{"rotate", "production", "--config", manifest})

	var out bytes.Buffer
	parent.SetOut(&out)
	if err := parent.Execute(); err != nil {
		t.Fatalf("parent.Execute(): %v", err)
	}
	if !strings.Contains(out.String(), "Rotated keypair for group") {
		t.Errorf("parent.Execute() output = %q, want rotation summary", out.String())
	}
}

// TestOutputRotateReportsResults verifies the rotation summary output.
func TestOutputRotateReportsResults(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	//nolint:gosec // G101: mock test metadata, not hardcoded credentials.
	err := outputKeypairRotate(
		printer.NewPlain(&out, &errOut),
		secrets.RotateKeypairResult{
			PublicKeyLocation: "/workspace/secrets.yaml",
			Keypair: secrets.KeypairMetadata{
				Group:     "production",
				PublicKey: "age1newkey...",
			},
			Secrets: []secrets.SecretReference{
				{Group: "production", Key: "k1"},
				{Group: "production", Key: "k2"},
				{Group: "production", Key: "k3"},
			},
			PrivateKeyLocation: "/home/user/.config/envx/keys/production.age",
		},
	)
	if err != nil {
		t.Fatalf("outputRotate(): %v", err)
	}

	got := out.String()
	wants := []string{
		"production",
		"age1newkey...",
		"3 secrets",
		"/workspace/secrets.yaml",
		"/home/user/.config/envx/keys/production.age",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("outputRotate() = %q, want it to contain %q", got, want)
		}
	}
}
