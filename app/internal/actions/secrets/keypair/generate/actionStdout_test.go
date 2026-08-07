package generate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/spf13/cobra"
)

// -------------------------------------------------------------------------------------

// TestExecuteStdoutUsesConfiguredCipher verifies stdout generation uses the
// manifest algorithm directly and does not create managed secret files.
func TestExecuteStdoutUsesConfiguredCipher(t *testing.T) {
	t.Parallel()

	manifest := filepath.Join(t.TempDir(), "envx.yaml")
	body := "environments: [production]\n" +
		"secrets:\n  cipher: nacl-box\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(manifest, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := executeStdout(
		actionParams{Group: "Production"},
		&config.Input{ConfigPath: &manifest},
	)
	if err != nil {
		t.Fatalf("executeStdout(): %v", err)
	}
	if !strings.HasPrefix(result.Keypair.PublicKey, "nacl-box-public-key:") {
		t.Errorf("public key = %q, want NaCl Box key", result.Keypair.PublicKey)
	}
	if !strings.HasPrefix(result.Keypair.PrivateKey, "nacl-box-private-key:") {
		t.Errorf("private key = %q, want NaCl Box key", result.Keypair.PrivateKey)
	}

	var output bytes.Buffer
	if err := renderStdout(&output, result); err != nil {
		t.Fatalf("renderStdout(): %v", err)
	}
	if !strings.Contains(output.String(), "group: production\n") ||
		!strings.Contains(output.String(), result.Keypair.PublicKey) ||
		!strings.Contains(output.String(), result.Keypair.PrivateKey) {
		t.Errorf("stdout output = %q", output.String())
	}

	for _, path := range []string{
		filepath.Join(filepath.Dir(manifest), "secrets.yaml"),
		filepath.Join(filepath.Dir(manifest), "envx.keys"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("stdout generation created %s, stat error = %v", path, err)
		}
	}
}

// -------------------------------------------------------------------------------------

// TestExecuteStdoutWithoutManifestUsesDefaultCipher verifies standalone stdout
// generation does not require an envx.yaml file.
func TestExecuteStdoutWithoutManifestUsesDefaultCipher(t *testing.T) {
	t.Parallel()

	missingManifest := filepath.Join(t.TempDir(), "envx.yaml")
	result, err := executeStdout(
		actionParams{},
		&config.Input{ConfigPath: &missingManifest},
	)
	if err != nil {
		t.Fatalf("executeStdout(): %v", err)
	}
	if !strings.HasPrefix(result.Keypair.PublicKey, "age-public-key:") {
		t.Errorf("public key = %q, want default Age key", result.Keypair.PublicKey)
	}
	if !strings.HasPrefix(result.Keypair.PrivateKey, "age-private-key:") {
		t.Errorf("private key = %q, want default Age key", result.Keypair.PrivateKey)
	}
}

// -------------------------------------------------------------------------------------

// TestRunStdoutWritesToCommandStreams verifies the stdout runner keeps key
// material on stdout and unassigned-key guidance on stderr.
func TestRunStdoutWritesToCommandStreams(t *testing.T) {
	manifest := writeManifest(t)
	var output bytes.Buffer
	var errors bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	cmd.SetErr(&errors)

	err := runStdout(
		cmd,
		actionParams{},
		&config.Input{ConfigPath: &manifest},
	)
	if err != nil {
		t.Fatalf("runStdout(): %v", err)
	}
	if !strings.Contains(output.String(), "public_key:") ||
		strings.Contains(output.String(), "No group provided") {
		t.Errorf("stdout = %q, want key material only", output.String())
	}
	if !strings.Contains(errors.String(), "No group provided") {
		t.Errorf("stderr = %q, want no-storage guidance", errors.String())
	}
}
