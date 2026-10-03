package cli

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/emit"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/test/fixtures"
)

// mockEmitFactory implements Factory for unit testing.
type mockEmitFactory struct {
	err     error
	emitErr error
}

func (m *mockEmitFactory) EnvService(configPath string) (*env.Service, error) {
	if m.err != nil {
		return nil, m.err
	}
	app, err := core.NewAppFactory()
	if err != nil {
		return nil, err
	}
	return app.EnvService(configPath)
}

func (m *mockEmitFactory) EmitService() (*emit.Service, error) {
	if m.emitErr != nil {
		return nil, m.emitErr
	}
	app, err := core.NewAppFactory()
	if err != nil {
		return nil, err
	}
	return app.EmitService()
}

// executeEmit builds the emit command, executes it with args, and returns its
// captured stdout and stderr.
func executeEmit(
	t *testing.T, factory Factory, configPath string, args ...string,
) (stdout, stderr string, err error) {
	t.Helper()

	cmd := NewEmitCommand(factory)
	flags.Bind(cmd.PersistentFlags(), &flags.Config)
	// The root command silences usage for runtime errors; mirror it here so a
	// failed emit leaves stdout to the rendered output alone.
	cmd.SilenceUsage = true
	cmd.SetArgs(append([]string{"--config", configPath}, args...))

	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err = cmd.Execute()
	return out.String(), errBuf.String(), err
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
		"public-keys:\n  production: "+pair.PublicKey+
			"\nsecrets:\n  production:\n    db: "+ciphertext+"\n")

	return filepath.Join(dir, "envx.yaml"), pair.PrivateKey
}

// TestValidateNameUsage verifies --name is accepted on the k8s targets (optional,
// since it defaults to the project) and rejected on the plain targets.
func TestValidateNameUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  emit.Target
		nameSet bool
		wantErr bool
	}{
		{"k8s with name", emit.TargetK8s, true, false},
		{"k8s without name", emit.TargetK8s, false, false},
		{"k8s-bundle with name", emit.TargetK8sBundle, true, false},
		{"dotenv without name", emit.TargetDotenv, false, false},
		{"dotenv with name", emit.TargetDotenv, true, true},
		{"json with name", emit.TargetJSON, true, true},
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
		target  emit.Target
		key     string
		keySet  bool
		wantErr bool
	}{
		{"bundle with json key", emit.TargetK8sBundle, "config.json", true, false},
		{"bundle with env key", emit.TargetK8sBundle, "app.env", true, false},
		{"bundle with bad ext", emit.TargetK8sBundle, "config.yaml", true, true},
		{"bundle without key", emit.TargetK8sBundle, "", false, false},
		{"k8s with key", emit.TargetK8s, "config.json", true, true},
		{"json with key", emit.TargetJSON, "config.json", true, true},
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

// TestNewEmitCommandDotenv verifies emit resolves a project and renders its
// environment as dotenv to stdout.
func TestNewEmitCommandDotenv(t *testing.T) {
	t.Parallel()

	stdout, _, err := executeEmit(
		t, &mockEmitFactory{}, fixtures.Manifest("basic"),
		"api-core", "--target", "dotenv",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if !strings.Contains(stdout, "APP_NAME=api-core\n") {
		t.Errorf("dotenv output missing APP_NAME:\n%s", stdout)
	}
}

// TestNewEmitCommandJSON verifies the json target renders a resolved environment
// as a JSON object.
func TestNewEmitCommandJSON(t *testing.T) {
	t.Parallel()

	stdout, _, err := executeEmit(
		t, &mockEmitFactory{}, fixtures.Manifest("basic"),
		"api-core", "--target", "json",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if !strings.Contains(stdout, "\"APP_NAME\": \"api-core\"") {
		t.Errorf("json output missing APP_NAME:\n%s", stdout)
	}
}

// TestNewEmitCommandRequiresTarget verifies emit rejects an invocation with no
// --target.
func TestNewEmitCommandRequiresTarget(t *testing.T) {
	t.Parallel()

	_, _, err := executeEmit(t, &mockEmitFactory{}, fixtures.Manifest("basic"), "api-core")
	if err == nil {
		t.Fatal("expected emit to require --target")
	}
}

// TestNewEmitCommandUnknownTarget verifies an unknown --target fails with the
// domain sentinel before any resolution work.
func TestNewEmitCommandUnknownTarget(t *testing.T) {
	t.Parallel()

	stdout, _, err := executeEmit(
		t, &mockEmitFactory{err: errors.New("must not resolve")},
		fixtures.Manifest("basic"), "api-core", "--target", "toml",
	)
	if !errors.Is(err, emit.ErrUnknownTarget) {
		t.Fatalf("got err = %v, want ErrUnknownTarget", err)
	}
	if stdout != "" {
		t.Errorf("emit wrote output despite the failure: %q", stdout)
	}
}

// TestNewEmitCommandFactoryError verifies an environment service failure aborts
// emit before any output is written.
func TestNewEmitCommandFactoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("factory failure")
	stdout, _, err := executeEmit(
		t, &mockEmitFactory{err: expectedErr}, "envx.yaml",
		"api-core", "--target", "dotenv",
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("got err = %v, want %v", err, expectedErr)
	}
	if stdout != "" {
		t.Errorf("emit wrote output despite the factory failure: %q", stdout)
	}
}

// TestNewEmitCommandEmitFactoryError verifies an emit service failure aborts
// emit before any output is written.
func TestNewEmitCommandEmitFactoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("emit failure")
	stdout, _, err := executeEmit(
		t, &mockEmitFactory{emitErr: expectedErr}, fixtures.Manifest("basic"),
		"api-core", "--target", "dotenv",
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("got err = %v, want %v", err, expectedErr)
	}
	if stdout != "" {
		t.Errorf("emit wrote output despite the factory failure: %q", stdout)
	}
}

// TestNewEmitCommandRejectsMisusedName verifies --name is rejected on a target
// that has no resource name.
func TestNewEmitCommandRejectsMisusedName(t *testing.T) {
	t.Parallel()

	_, _, err := executeEmit(
		t, &mockEmitFactory{}, fixtures.Manifest("basic"),
		"api-core", "--target", "dotenv", "--name", "oops",
	)
	if err == nil {
		t.Fatal("expected --name on dotenv to be rejected")
	}
}

// TestNewEmitCommandK8sDefaultsNameToProject verifies the k8s target renders
// without --name, deriving the resource name from the project.
func TestNewEmitCommandK8sDefaultsNameToProject(t *testing.T) {
	t.Parallel()

	stdout, _, err := executeEmit(
		t, &mockEmitFactory{}, fixtures.Manifest("basic"),
		"api-core", "--target", "k8s", "--only", "config",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if !strings.Contains(stdout, "name: api-core-config") {
		t.Errorf("ConfigMap should be named after the project:\n%s", stdout)
	}
}

// TestNewEmitCommandFailsClosedOnUnresolved verifies an unresolved value aborts
// emit before any output is written, so no partial render escapes.
func TestNewEmitCommandFailsClosedOnUnresolved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeWorkspaceFile(t, dir, "envx.yaml",
		"environments: [development]\nprojects:\n  api:\n    includes: [env/app]\n")
	writeWorkspaceFile(t, dir, filepath.Join("env", "app.yaml"),
		"good: value\nbroken: \"{{NOPE}}\"\n")

	stdout, _, err := executeEmit(
		t, &mockEmitFactory{}, filepath.Join(dir, "envx.yaml"),
		"api", "--target", "dotenv",
	)
	if err == nil {
		t.Fatal("expected an unresolved value to abort emit")
	}
	if !strings.Contains(err.Error(), "BROKEN") {
		t.Errorf("error %q should name the unresolved key", err)
	}
	if stdout != "" {
		t.Errorf("emit wrote output despite the failure: %q", stdout)
	}
}

// TestNewEmitCommandK8sSecretRevealsOnlySecrets verifies the k8s target with only
// the secret slice selected renders a Secret carrying the decrypted
// secret-derived value (base64-encoded) and not the plain one.
func TestNewEmitCommandK8sSecretRevealsOnlySecrets(t *testing.T) {
	cfgPath, privateKey := encryptedWorkspace(t)
	t.Setenv("ENVX_PRIVATE_KEY_PRODUCTION", privateKey)

	stdout, _, err := executeEmit(
		t, &mockEmitFactory{}, cfgPath,
		"api", "--target", "k8s", "--name", "api-secrets", "--only", "secrets",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}

	if !strings.Contains(stdout, "kind: Secret") ||
		!strings.Contains(stdout, "name: api-secrets") {
		t.Errorf("output is not the expected Secret:\n%s", stdout)
	}
	wantData := "DB_PASSWORD: " + base64.StdEncoding.EncodeToString([]byte("top-secret"))
	if !strings.Contains(stdout, wantData) {
		t.Errorf("Secret missing the decrypted secret %q:\n%s", wantData, stdout)
	}
	if strings.Contains(stdout, "APP_NAME") {
		t.Errorf("Secret unexpectedly carries the plain value APP_NAME:\n%s", stdout)
	}
	// The plaintext secret must never appear unencoded in the manifest.
	if strings.Contains(stdout, "top-secret") {
		t.Errorf("Secret leaked plaintext secret material:\n%s", stdout)
	}
}

// TestNewEmitCommandK8sConfigMapExcludesSecrets verifies the k8s target with only
// the config slice selected renders a ConfigMap carrying the plain value and not
// the secret-derived one.
func TestNewEmitCommandK8sConfigMapExcludesSecrets(t *testing.T) {
	cfgPath, privateKey := encryptedWorkspace(t)
	t.Setenv("ENVX_PRIVATE_KEY_PRODUCTION", privateKey)

	stdout, _, err := executeEmit(
		t, &mockEmitFactory{}, cfgPath,
		"api", "--target", "k8s", "--name", "api-config", "--only", "config",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}

	if !strings.Contains(stdout, "kind: ConfigMap") ||
		!strings.Contains(stdout, "name: api-config") {
		t.Errorf("output is not the expected ConfigMap:\n%s", stdout)
	}
	if !strings.Contains(stdout, "APP_NAME: myapp") {
		t.Errorf("ConfigMap missing the plain value APP_NAME:\n%s", stdout)
	}
	for _, secret := range []string{"DB_PASSWORD", "top-secret"} {
		if strings.Contains(stdout, secret) {
			t.Errorf("ConfigMap unexpectedly carries secret material %q:\n%s", secret, stdout)
		}
	}
}

// TestNewEmitCommandMissingKeyAborts verifies that when the private key is
// unavailable the secret cannot be revealed, so emit fails closed with no output.
func TestNewEmitCommandMissingKeyAborts(t *testing.T) {
	cfgPath, _ := encryptedWorkspace(t)
	// Deliberately do not export ENVX_PRIVATE_KEY_PRODUCTION.

	stdout, _, err := executeEmit(
		t, &mockEmitFactory{}, cfgPath,
		"api", "--target", "k8s", "--name", "api-secrets", "--only", "secrets",
	)
	if err == nil {
		t.Fatal("expected emit to fail when the private key is unavailable")
	}
	if stdout != "" {
		t.Errorf("emit wrote output despite the reveal failure: %q", stdout)
	}
}

// TestNewEmitCommandWritesFile verifies emit writes to a file with private
// permissions instead of stdout when an output path is given.
func TestNewEmitCommandWritesFile(t *testing.T) {
	t.Parallel()

	outPath := filepath.Join(t.TempDir(), "out.env")
	stdout, _, err := executeEmit(
		t, &mockEmitFactory{}, fixtures.Manifest("basic"),
		"api-core", "--target", "dotenv", "--output", outPath,
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if strings.Contains(stdout, "APP_NAME=") {
		t.Errorf("emit wrote the render to stdout despite an output path: %q", stdout)
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

// TestNewEmitCommandWarnsOnSecretFile verifies a file target whose output carries
// secret material warns on stderr — preceded by a blank line — that the file must
// not be committed, while a ConfigMap file (which excludes secrets) does not warn.
func TestNewEmitCommandWarnsOnSecretFile(t *testing.T) {
	cfgPath, privateKey := encryptedWorkspace(t)
	t.Setenv("ENVX_PRIVATE_KEY_PRODUCTION", privateKey)

	// A dotenv file carries the decrypted secret, so it must warn. Color is off
	// (a buffer is not a terminal), so the WARNING label appears without glyph or
	// escape codes.
	outPath := filepath.Join(t.TempDir(), "out.env")
	stdout, stderr, err := executeEmit(
		t, &mockEmitFactory{}, cfgPath,
		"api", "--target", "dotenv", "--output", outPath,
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	// The confirmation is normal output (stdout); the caution is a stderr warning.
	if !strings.Contains(stdout, "Wrote api ") || !strings.Contains(stdout, outPath) {
		t.Errorf("expected a wrote-confirmation on stdout naming %q, got %q", outPath, stdout)
	}
	if !strings.Contains(stderr, "WARNING:") ||
		!strings.Contains(stderr, "do not commit") {
		t.Errorf("expected a do-not-commit warning on stderr, got %q", stderr)
	}

	// A config-only file excludes secrets, so it confirms the write but does not
	// warn.
	cfgOut := filepath.Join(t.TempDir(), "config.yaml")
	stdout, stderr, err = executeEmit(
		t, &mockEmitFactory{}, cfgPath,
		"api", "--target", "k8s", "--name", "api-config", "--only", "config",
		"--output", cfgOut,
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if !strings.Contains(stdout, "Wrote api config to") {
		t.Errorf("config-only file should confirm the write on stdout, got %q", stdout)
	}
	if strings.Contains(stderr, "WARNING:") {
		t.Errorf("config-only file should not warn, got %q", stderr)
	}
}

// TestNewEmitCommandStdoutStaysQuiet verifies emitting to stdout prints nothing on
// stderr, so a piped manifest is clean.
func TestNewEmitCommandStdoutStaysQuiet(t *testing.T) {
	cfgPath, privateKey := encryptedWorkspace(t)
	t.Setenv("ENVX_PRIVATE_KEY_PRODUCTION", privateKey)

	_, stderr, err := executeEmit(
		t, &mockEmitFactory{}, cfgPath,
		"api", "--target", "k8s-bundle", "--only", "config",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if stderr != "" {
		t.Errorf("emit to stdout should print nothing on stderr, got %q", stderr)
	}
}

// TestNewEmitCommandMissingOutputDir verifies emit refuses to write into a
// directory that does not exist, failing before any output rather than creating
// it.
func TestNewEmitCommandMissingOutputDir(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "nope", "out.env")
	_, _, err := executeEmit(
		t, &mockEmitFactory{}, fixtures.Manifest("basic"),
		"api-core", "--target", "dotenv", "--output", missing,
	)
	if err == nil {
		t.Fatal("expected an error for a missing output directory")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error %q should explain the directory is missing", err)
	}
}
