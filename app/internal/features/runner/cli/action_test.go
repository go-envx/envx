package cli

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/fixtures"
	"github.com/go-envx/envx/app/internal/resources/cipher"
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

// TestExecuteUnionsOSKeys verifies the child receives OS-only environment
// variables too, so the effective environment stays complete now that
// Materialize (not the runner) composes it.
func TestExecuteUnionsOSKeys(t *testing.T) {
	t.Setenv("OS_ONLY_VAR", "present")

	path := fixtures.Manifest("basic")
	var stdout bytes.Buffer
	in := &config.Input{ConfigPath: &path}
	err := execute(actionParams{
		Project:  "api-core",
		ExecArgs: []string{"printenv", "OS_ONLY_VAR"},
	}, in, streams{Stdout: &stdout, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := stdout.String(); got != "present\n" {
		t.Errorf("child OS_ONLY_VAR = %q, want present", got)
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

// TestExecuteIgnoreErrorsFailsClosedByDefault verifies that without
// --ignore-errors an unresolved reference aborts the run before the child starts.
func TestExecuteIgnoreErrorsFailsClosedByDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeWorkspaceFile(t, dir, "envx.yaml",
		"environments: [development]\nprojects:\n  api:\n    includes: [env/app]\n")
	writeWorkspaceFile(t, dir, filepath.Join("env", "app.yaml"),
		"good: value\nbroken: \"{{NOPE}}\"\n")

	cfgPath := filepath.Join(dir, "envx.yaml")
	var stdout bytes.Buffer
	err := execute(actionParams{
		Project:  "api",
		ExecArgs: []string{"printenv", "GOOD"},
	}, &config.Input{ConfigPath: &cfgPath}, streams{Stdout: &stdout, Stderr: io.Discard})
	if err == nil {
		t.Fatal("expected the missing reference to abort the run")
	}
	if stdout.Len() != 0 {
		t.Errorf("child produced output %q despite the failure", stdout.String())
	}
}

// TestExecuteIgnoreErrorsStartsChild verifies --ignore-errors downgrades an
// unresolved reference to a stderr warning, omits its key (leaving it unset, not
// empty), and still starts the child with the keys that did resolve.
func TestExecuteIgnoreErrorsStartsChild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeWorkspaceFile(t, dir, "envx.yaml",
		"environments: [development]\nprojects:\n  api:\n    includes: [env/app]\n")
	writeWorkspaceFile(t, dir, filepath.Join("env", "app.yaml"),
		"good: value\nbroken: \"{{NOPE}}\"\n")

	cfgPath := filepath.Join(dir, "envx.yaml")
	var stdout, stderr bytes.Buffer
	err := execute(actionParams{
		Project:      "api",
		ExecArgs:     []string{"sh", "-c", "echo GOOD=$GOOD; echo BROKEN=${BROKEN-<unset>}"},
		IgnoreErrors: true,
	}, &config.Input{ConfigPath: &cfgPath}, streams{Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// GOOD resolves; BROKEN is omitted entirely, so the child sees it as unset
	// rather than an empty string.
	if got := stdout.String(); got != "GOOD=value\nBROKEN=<unset>\n" {
		t.Errorf("child output = %q, want GOOD=value + BROKEN unset", got)
	}
	warned := stderr.String()
	if !strings.Contains(warned, "WARNING") || !strings.Contains(warned, "BROKEN") {
		t.Errorf("stderr = %q, want a warning naming BROKEN", warned)
	}
}

// TestExecuteIgnoreErrorsKeepsAmbientValue verifies that under --overload a broken
// file value which the shell already defines is not omitted: the ambient value
// survives so the file value never clobbers it.
func TestExecuteIgnoreErrorsKeepsAmbientValue(t *testing.T) {
	t.Setenv("BROKEN", "from-shell")
	t.Setenv("ENVX_OVERLOAD", "true")

	dir := t.TempDir()
	writeWorkspaceFile(t, dir, "envx.yaml",
		"environments: [development]\nprojects:\n  api:\n    includes: [env/app]\n")
	writeWorkspaceFile(t, dir, filepath.Join("env", "app.yaml"),
		"broken: \"{{MISSING}}\"\n")

	cfgPath := filepath.Join(dir, "envx.yaml")
	var stdout, stderr bytes.Buffer
	err := execute(actionParams{
		Project:      "api",
		ExecArgs:     []string{"sh", "-c", "echo BROKEN=$BROKEN"},
		IgnoreErrors: true,
	}, &config.Input{ConfigPath: &cfgPath}, streams{Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := stdout.String(); got != "BROKEN=from-shell\n" {
		t.Errorf("child BROKEN = %q, want the ambient from-shell", got)
	}
	if !strings.Contains(stderr.String(), "keeping the value") {
		t.Errorf("stderr = %q, want a fallback warning", stderr.String())
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
