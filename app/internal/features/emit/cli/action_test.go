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
	engine "github.com/go-envx/envx/app/internal/features/emit"
	"github.com/go-envx/envx/app/internal/fixtures"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/utils/printer"
)

// discardPrinter returns a Printer whose streams are discarded, for the cases
// that assert on the rendered output rather than the warning.
func discardPrinter() *printer.Printer {
	return printer.New(printer.Options{Out: io.Discard, Err: io.Discard})
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

// TestValidateNameUsage verifies --name is accepted on the k8s targets (optional,
// since it defaults to the project) and rejected on the plain targets.
func TestValidateNameUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  engine.Target
		nameSet bool
		wantErr bool
	}{
		{"k8s with name", engine.TargetK8s, true, false},
		{"k8s without name", engine.TargetK8s, false, false},
		{"k8s-bundle with name", engine.TargetK8sBundle, true, false},
		{"dotenv without name", engine.TargetDotenv, false, false},
		{"dotenv with name", engine.TargetDotenv, true, true},
		{"json with name", engine.TargetJSON, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateNameUsage(tt.target, tt.nameSet)
			if tt.wantErr != (err != nil) {
				t.Errorf("validateNameUsage(%s, %v) error = %v, wantErr %v",
					tt.target, tt.nameSet, err, tt.wantErr)
			}
		})
	}
}

// TestValidateKeyUsage verifies --key is accepted only on k8s-bundle, rejected on
// the other targets, and rejected when its extension names no known format.
func TestValidateKeyUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  engine.Target
		key     string
		keySet  bool
		wantErr bool
	}{
		{"bundle with json key", engine.TargetK8sBundle, "config.json", true, false},
		{"bundle with env key", engine.TargetK8sBundle, "app.env", true, false},
		{"bundle with bad ext", engine.TargetK8sBundle, "config.yaml", true, true},
		{"bundle without key", engine.TargetK8sBundle, "", false, false},
		{"k8s with key", engine.TargetK8s, "config.json", true, true},
		{"json with key", engine.TargetJSON, "config.json", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateKeyUsage(tt.target, tt.key, tt.keySet)
			if tt.wantErr != (err != nil) {
				t.Errorf("validateKeyUsage(%s, %q, %v) error = %v, wantErr %v",
					tt.target, tt.key, tt.keySet, err, tt.wantErr)
			}
		})
	}
}

// TestResolveSlice verifies the --only value maps to the right slice selectors:
// empty selects both, the named slices select one, and anything else is rejected.
func TestResolveSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		only        string
		wantSecrets bool
		wantConfig  bool
		wantErr     bool
	}{
		{"", true, true, false},
		{"secrets", true, false, false},
		{"config", false, true, false},
		{"plain", false, false, true},
		{"both", false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.only, func(t *testing.T) {
			t.Parallel()
			gotSecrets, gotConfig, err := resolveSlice(tt.only)
			if tt.wantErr {
				if err == nil {
					t.Errorf("resolveSlice(%q) = nil error, want error", tt.only)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSlice(%q): %v", tt.only, err)
			}
			if gotSecrets != tt.wantSecrets || gotConfig != tt.wantConfig {
				t.Errorf("resolveSlice(%q) = (%v, %v), want (%v, %v)",
					tt.only, gotSecrets, gotConfig, tt.wantSecrets, tt.wantConfig)
			}
		})
	}
}

// TestExecuteDotenv verifies emit resolves a project and renders its environment
// as dotenv to the provided writer.
func TestExecuteDotenv(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	var stdout bytes.Buffer
	err := execute(actionParams{
		Project: "api-core",
		Target:  engine.TargetDotenv,
	}, &config.Input{ConfigPath: &path}, &stdout, discardPrinter())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "APP_NAME=api-core\n") {
		t.Errorf("dotenv output missing APP_NAME:\n%s", got)
	}
}

// TestExecuteJSON verifies the json target renders a resolved environment as a
// JSON object.
func TestExecuteJSON(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	var stdout bytes.Buffer
	err := execute(actionParams{
		Project: "api-core",
		Target:  engine.TargetJSON,
	}, &config.Input{ConfigPath: &path}, &stdout, discardPrinter())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "\"APP_NAME\": \"api-core\"") {
		t.Errorf("json output missing APP_NAME:\n%s", got)
	}
}

// TestExecuteFailsClosedOnUnresolved verifies an unresolved value aborts emit
// before any output is written, so no partial render escapes.
func TestExecuteFailsClosedOnUnresolved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeWorkspaceFile(t, dir, "envx.yaml",
		"environments: [development]\nprojects:\n  api:\n    includes: [env/app]\n")
	writeWorkspaceFile(t, dir, filepath.Join("env", "app.yaml"),
		"good: value\nbroken: \"{{NOPE}}\"\n")

	cfgPath := filepath.Join(dir, "envx.yaml")
	var stdout bytes.Buffer
	err := execute(actionParams{
		Project: "api",
		Target:  engine.TargetDotenv,
	}, &config.Input{ConfigPath: &cfgPath}, &stdout, discardPrinter())
	if err == nil {
		t.Fatal("expected an unresolved value to abort emit")
	}
	if !strings.Contains(err.Error(), "BROKEN") {
		t.Errorf("error %q should name the unresolved key", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("emit wrote output despite the failure: %q", stdout.String())
	}
}

// encryptedWorkspace writes a workspace that references one encrypted secret and
// one plain value, returning the config path and the private key that decrypts
// the secret. The store ships the public key so the workspace is complete; the
// private key is returned to the caller to supply via the environment.
func encryptedWorkspace(t *testing.T) (cfgPath, privateKey string) {
	t.Helper()
	dir := t.TempDir()

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
		"environments: [production]\nprojects:\n  api:\n    includes: [env/app]\n")
	writeWorkspaceFile(t, dir, filepath.Join("env", "app.yaml"),
		"app_name: myapp\ndb_password: secret://production/db\n")
	writeWorkspaceFile(t, dir, "secrets.yaml",
		"public_keys:\n  production: "+pair.PublicKey+
			"\nsecrets:\n  production:\n    db: "+ciphertext+"\n")

	return filepath.Join(dir, "envx.yaml"), pair.PrivateKey
}

// TestExecuteK8sSecretRevealsOnlySecrets verifies the k8s-secret target renders a
// Secret carrying the decrypted secret-derived value (base64-encoded) and not the
// plain one.
func TestExecuteK8sSecretRevealsOnlySecrets(t *testing.T) {
	cfgPath, privateKey := encryptedWorkspace(t)
	t.Setenv("ENVX_PRIVATE_KEY_PRODUCTION", privateKey)

	var stdout bytes.Buffer
	err := execute(actionParams{
		Project:        "api",
		Target:         engine.TargetK8s,
		Name:           "api-secrets",
		IncludeSecrets: true,
	}, &config.Input{ConfigPath: &cfgPath}, &stdout, discardPrinter())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()

	if !strings.Contains(got, "kind: Secret") ||
		!strings.Contains(got, "name: api-secrets") {
		t.Errorf("output is not the expected Secret:\n%s", got)
	}
	wantData := "DB_PASSWORD: " + base64.StdEncoding.EncodeToString([]byte("top-secret"))
	if !strings.Contains(got, wantData) {
		t.Errorf("Secret missing the decrypted, base64-encoded secret %q:\n%s", wantData, got)
	}
	if strings.Contains(got, "APP_NAME") {
		t.Errorf("Secret unexpectedly carries the plain value APP_NAME:\n%s", got)
	}
	// The plaintext secret must never appear unencoded in the manifest.
	if strings.Contains(got, "top-secret") {
		t.Errorf("Secret leaked plaintext secret material:\n%s", got)
	}
}

// TestExecuteK8sConfigMapExcludesSecrets verifies the k8s target with only the
// config slice selected renders a ConfigMap carrying the plain value and not the
// secret-derived one.
func TestExecuteK8sConfigMapExcludesSecrets(t *testing.T) {
	cfgPath, privateKey := encryptedWorkspace(t)
	t.Setenv("ENVX_PRIVATE_KEY_PRODUCTION", privateKey)

	var stdout bytes.Buffer
	err := execute(actionParams{
		Project:       "api",
		Target:        engine.TargetK8s,
		Name:          "api-config",
		IncludeConfig: true,
	}, &config.Input{ConfigPath: &cfgPath}, &stdout, discardPrinter())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()

	if !strings.Contains(got, "kind: ConfigMap") ||
		!strings.Contains(got, "name: api-config") {
		t.Errorf("output is not the expected ConfigMap:\n%s", got)
	}
	if !strings.Contains(got, "APP_NAME: myapp") {
		t.Errorf("ConfigMap missing the plain value APP_NAME:\n%s", got)
	}
	for _, secret := range []string{"DB_PASSWORD", "top-secret"} {
		if strings.Contains(got, secret) {
			t.Errorf("ConfigMap unexpectedly carries secret material %q:\n%s", secret, got)
		}
	}
}

// TestExecuteMissingKeyAborts verifies that when the private key is unavailable
// the secret cannot be revealed, so emit fails closed with no output.
func TestExecuteMissingKeyAborts(t *testing.T) {
	cfgPath, _ := encryptedWorkspace(t)
	// Deliberately do not export ENVX_PRIVATE_KEY_PRODUCTION.

	var stdout bytes.Buffer
	err := execute(actionParams{
		Project:        "api",
		Target:         engine.TargetK8s,
		Name:           "api-secrets",
		IncludeSecrets: true,
	}, &config.Input{ConfigPath: &cfgPath}, &stdout, discardPrinter())
	if err == nil {
		t.Fatal("expected emit to fail when the private key is unavailable")
	}
	if stdout.Len() != 0 {
		t.Errorf("emit wrote output despite the reveal failure: %q", stdout.String())
	}
}

// TestExecuteWritesFile verifies emit writes to a file with private permissions
// instead of stdout when an output path is given.
func TestExecuteWritesFile(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	outPath := filepath.Join(t.TempDir(), "out.env")
	var stdout bytes.Buffer
	err := execute(actionParams{
		Project:    "api-core",
		Target:     engine.TargetDotenv,
		OutputPath: outPath,
	}, &config.Input{ConfigPath: &path}, &stdout, discardPrinter())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("emit wrote to stdout despite an output path: %q", stdout.String())
	}

	//nolint:gosec // test-local path.
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", outPath, err)
	}
	if !strings.Contains(string(data), "APP_NAME=api-core\n") {
		t.Errorf("output file missing APP_NAME:\n%s", data)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("Stat(%q): %v", outPath, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("output file mode = %o, want 600 (secrets may be present)", perm)
	}
}

// TestExecuteWarnsOnSecretFile verifies a file target whose output carries secret
// material warns on stderr — preceded by a blank line — that the file must not be
// committed, while a ConfigMap file (which excludes secrets) does not warn.
func TestExecuteWarnsOnSecretFile(t *testing.T) {
	cfgPath, privateKey := encryptedWorkspace(t)
	t.Setenv("ENVX_PRIVATE_KEY_PRODUCTION", privateKey)

	// A dotenv file carries the decrypted secret, so it must warn. Color is off
	// (a buffer is not a terminal), so the WARNING label appears without glyph or
	// escape codes.
	outPath := filepath.Join(t.TempDir(), "out.env")
	var stdout, stderr bytes.Buffer
	pr := printer.New(printer.Options{Out: &stdout, Err: &stderr})
	if err := execute(actionParams{
		Project:    "api",
		Target:     engine.TargetDotenv,
		OutputPath: outPath,
	}, &config.Input{ConfigPath: &cfgPath}, &stdout, pr); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// The confirmation is normal output (stdout); the caution is a stderr warning.
	if got := stdout.String(); !strings.Contains(got, "Wrote api ") ||
		!strings.Contains(got, outPath) {
		t.Errorf("expected a wrote-confirmation on stdout naming %q, got %q", outPath, got)
	}
	if got := stderr.String(); !strings.Contains(got, "WARNING:") ||
		!strings.Contains(got, "do not commit") {
		t.Errorf("expected a do-not-commit warning on stderr, got %q", got)
	}

	// A config-only file excludes secrets, so it confirms the write but does not
	// warn.
	cfgOut := filepath.Join(t.TempDir(), "config.yaml")
	stdout.Reset()
	stderr.Reset()
	if err := execute(actionParams{
		Project:       "api",
		Target:        engine.TargetK8s,
		Name:          "api-config",
		IncludeConfig: true,
		OutputPath:    cfgOut,
	}, &config.Input{ConfigPath: &cfgPath}, &stdout, pr); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "Wrote api config to") {
		t.Errorf("config-only file should confirm the write on stdout, got %q", got)
	}
	if got := stderr.String(); strings.Contains(got, "WARNING:") {
		t.Errorf("config-only file should not warn, got %q", got)
	}
}

// TestExecuteStdoutStaysQuiet verifies emitting to stdout prints nothing on
// stderr, so a piped manifest is clean.
func TestExecuteStdoutStaysQuiet(t *testing.T) {
	cfgPath, privateKey := encryptedWorkspace(t)
	t.Setenv("ENVX_PRIVATE_KEY_PRODUCTION", privateKey)

	var stdout, stderr bytes.Buffer
	pr := printer.New(printer.Options{Out: &stdout, Err: &stderr})
	if err := execute(actionParams{
		Project:       "api",
		Target:        engine.TargetK8sBundle,
		IncludeConfig: true,
	}, &config.Input{ConfigPath: &cfgPath}, &stdout, pr); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("emit to stdout should print nothing on stderr, got %q", stderr.String())
	}
}

// TestExecuteMissingOutputDir verifies emit refuses to write into a directory
// that does not exist, failing before any output rather than creating it.
func TestExecuteMissingOutputDir(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	missing := filepath.Join(t.TempDir(), "nope", "out.env")
	var stdout bytes.Buffer
	err := execute(actionParams{
		Project:    "api-core",
		Target:     engine.TargetDotenv,
		OutputPath: missing,
	}, &config.Input{ConfigPath: &path}, &stdout, discardPrinter())
	if err == nil {
		t.Fatal("expected an error for a missing output directory")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error %q should explain the directory is missing", err)
	}
}
