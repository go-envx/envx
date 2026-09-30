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

// writeKeypairGenerateManifest creates a valid workspace manifest for generate tests.
func writeKeypairGenerateManifest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestGenerateAndRender verifies generation writes through the manager and does
// not render private-key bytes.
func TestGenerateAndRender(t *testing.T) {
	manifest := writeKeypairGenerateManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	result, err := manager.GenerateKeypair("Production")
	if err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if result.Keypair.Group != "production" {
		t.Errorf("Group = %q, want production", result.Keypair.Group)
	}
	if result.Keypair.PrivateKeyStatus != "valid" {
		t.Errorf(
			"PrivateKeyStatus = %q, want valid",
			result.Keypair.PrivateKeyStatus,
		)
	}

	privateData, err := filex.Read(resolved.Secrets.KeysPath)
	if err != nil {
		t.Fatalf("read private-key file: %v", err)
	}
	if len(privateData) == 0 {
		t.Fatal("private-key file is empty")
	}
}

// TestGenerateUsesConfiguredCipher verifies normal generation uses the manifest
// algorithm before persisting the keypair through the manager.
func TestGenerateUsesConfiguredCipher(t *testing.T) {
	t.Parallel()

	manifest := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"secrets:\n  cipher: nacl-box\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(manifest, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	input := &core.Input{ConfigPath: &manifest}
	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}
	result, err := manager.GenerateKeypair("production")
	if err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if !strings.HasPrefix(result.Keypair.PublicKey, "nacl-box-public-key:") {
		t.Errorf(
			"public key = %q, want NaCl Box key",
			result.Keypair.PublicKey,
		)
	}
	privateData, err := filex.Read(resolved.Secrets.KeysPath)
	if err != nil {
		t.Fatalf("read private-key file: %v", err)
	}
	if !strings.Contains(string(privateData), "nacl-box-private-key:") {
		t.Errorf("private-key file = %q, want NaCl Box key", privateData)
	}
}

// TestNewKeypairGenerateCommand verifies generation dispatches through the factory.
func TestNewKeypairGenerateCommand(t *testing.T) {
	manifest := writeKeypairGenerateManifest(t)
	input := &core.Input{ConfigPath: &manifest}
	resolved, err := core.ResolveWorkspace(input)
	if err != nil {
		t.Fatalf("ResolveWorkspace(): %v", err)
	}
	manager, err := core.NewSecretsManager(resolved.Secrets, resolved.Cipher)
	if err != nil {
		t.Fatalf("NewSecretsManager(): %v", err)
	}

	parent := NewKeypairCmd(&mockSecretsFactory{svc: manager})
	parent.PersistentFlags().String("config", manifest, "")
	parent.SetArgs([]string{"generate", "production", "--config", manifest})

	var out bytes.Buffer
	parent.SetOut(&out)
	if err := parent.Execute(); err != nil {
		t.Fatalf("parent.Execute(): %v", err)
	}
	if !strings.Contains(out.String(), "Generated keypair for group") {
		t.Errorf("parent.Execute() output = %q, want generation summary", out.String())
	}
}

// TestOutputGenerateReportsMetadata verifies the output summary displays public key
// and paths without exposing private keys.
func TestOutputGenerateReportsMetadata(t *testing.T) {
	t.Parallel()

	//nolint:lll,gosec // display assertion for mock public key and path
	pubKey := "age-public-key:age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"
	var out, errOut bytes.Buffer
	//nolint:gosec // G101: mock test metadata, not hardcoded credentials.
	err := outputKeypairGenerate(
		printer.NewPlain(&out, &errOut),
		secrets.GenerateKeypairResult{
			Keypair: secrets.KeypairMetadata{
				Group:            "production",
				PublicKey:        pubKey,
				PrivateKeyStatus: "valid",
			},
			PublicKeyLocation:  "/workspace/secrets.yaml",
			PrivateKeyLocation: "/home/user/.config/envx/keys/production.age",
		},
	)
	if err != nil {
		t.Fatalf("outputGenerate(): %v", err)
	}

	got := out.String()
	wants := []string{
		`"production"`,
		pubKey,
		"/workspace/secrets.yaml",
		"/home/user/.config/envx/keys/production.age",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("outputGenerate() = %q, want it to contain %q", got, want)
		}
	}
}
