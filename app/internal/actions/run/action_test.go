package run

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/fixtures"
)

// TestExecuteInjectsEnv verifies the resolved environment reaches the child
// process under the default (no-overload) settings.
func TestExecuteInjectsEnv(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	var stdout bytes.Buffer
	in := &config.Input{ConfigPath: &path}
	err := execute(actionParams{
		Project:  "api-core",
		ExecArgs: []string{"printenv", "APP_NAME"},
	}, in, streams{Stdout: &stdout, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := stdout.String(); got != "api-core\n" {
		t.Errorf("child APP_NAME = %q, want api-core", got)
	}
}

// TestExecuteOverloadFromEnv verifies ENVX_OVERLOAD lets file values win over an
// OS env var even without the --overload flag.
func TestExecuteOverloadFromEnv(t *testing.T) {
	t.Setenv("APP_NAME", "from-os")
	t.Setenv("ENVX_OVERLOAD", "true")

	path := fixtures.Manifest("basic")
	var stdout bytes.Buffer
	in := &config.Input{ConfigPath: &path}
	err := execute(actionParams{
		Project:  "api-core",
		ExecArgs: []string{"printenv", "APP_NAME"},
	}, in, streams{Stdout: &stdout, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := stdout.String(); got != "api-core\n" {
		t.Errorf("APP_NAME = %q, want api-core (file wins via ENVX_OVERLOAD)", got)
	}
}

// TestExecuteRevealFailurePreventsChildStartup verifies run reveals secrets
// before starting the child, so a reference it cannot decrypt fails during
// resolution and the child process never runs. The workspace stores real
// ciphertext but ships no private key, so the required key is unavailable.
func TestExecuteRevealFailurePreventsChildStartup(t *testing.T) {
	dir := t.TempDir()

	// Encrypt a value to a fresh keypair whose private key is never written, so
	// revealing the reference must fail for want of a key.
	selected, err := cipher.New(cipher.Params{Algorithm: cipher.Age})
	if err != nil {
		t.Fatalf("cipher.New(): %v", err)
	}
	pair, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	raw, err := selected.Encrypt("top-secret", pair.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt(): %v", err)
	}
	ciphertext := "encrypted-age:" + base64.RawURLEncoding.EncodeToString(raw)

	writeWorkspaceFile(t, dir, "envx.yaml",
		"environments: [development]\nprojects:\n  api:\n    includes: [env/app]\n")
	writeWorkspaceFile(t, dir, filepath.Join("env", "app.yaml"),
		"password: secret://production/db\n")
	writeWorkspaceFile(t, dir, "secrets.yaml",
		"public_keys:\n  production: "+pair.PublicKey+
			"\nsecrets:\n  production:\n    db: "+ciphertext+"\n")

	cfgPath := filepath.Join(dir, "envx.yaml")
	var stdout bytes.Buffer
	err = execute(actionParams{
		Project:  "api",
		ExecArgs: []string{"printenv", "PASSWORD"},
	}, &config.Input{ConfigPath: &cfgPath}, streams{Stdout: &stdout, Stderr: io.Discard})
	if err == nil {
		t.Fatal("expected the reveal failure to prevent child-process startup")
	}
	if stdout.Len() != 0 {
		t.Errorf(
			"child produced output %q despite the reveal failure", stdout.String(),
		)
	}
}

// writeWorkspaceFile writes body to a workspace-relative path under dir, creating
// parent directories as needed.
func writeWorkspaceFile(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
