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
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/runner"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/test/fixtures"
)

// mockRunFactory implements Factory for unit testing.
type mockRunFactory struct {
	err       error
	runnerErr error
}

func (m *mockRunFactory) EnvService(configPath string) (*env.Service, error) {
	if m.err != nil {
		return nil, m.err
	}
	app, err := core.NewAppFactory()
	if err != nil {
		return nil, err
	}
	return app.EnvService(configPath)
}

func (m *mockRunFactory) RunnerService() (*runner.Service, error) {
	if m.runnerErr != nil {
		return nil, m.runnerErr
	}
	app, err := core.NewAppFactory()
	if err != nil {
		return nil, err
	}
	return app.RunnerService()
}

// executeRun builds the run command, executes it with args, and returns its
// captured stdout and stderr.
func executeRun(
	t *testing.T, factory Factory, configPath string, args ...string,
) (stdout, stderr string, err error) {
	t.Helper()

	cmd := NewRunCommand(factory)
	flags.Bind(cmd.PersistentFlags(), &flags.Config)
	// The root command silences usage for runtime errors; mirror it here so a
	// failed run leaves stdout to the child alone.
	cmd.SilenceUsage = true
	cmd.SetArgs(append([]string{"--config", configPath}, args...))

	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetIn(strings.NewReader(""))

	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

// TestNewRunCommandInjectsEnv verifies the resolved environment reaches the child
// process under the default (no-overload) settings.
func TestNewRunCommandInjectsEnv(t *testing.T) {
	t.Parallel()

	path := fixtures.Manifest("basic")
	stdout, _, err := executeRun(
		t, &mockRunFactory{}, path, "api-core", "--", "printenv", "APP_NAME",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if stdout != "api-core\n" {
		t.Errorf("child APP_NAME = %q, want api-core", stdout)
	}
}

// TestNewRunCommandOverloadFromEnv verifies ENVX_OVERLOAD lets file values win
// over an OS env var even without the --overload flag.
func TestNewRunCommandOverloadFromEnv(t *testing.T) {
	t.Setenv("APP_NAME", "from-os")
	t.Setenv("ENVX_OVERLOAD", "true")

	path := fixtures.Manifest("basic")
	stdout, _, err := executeRun(
		t, &mockRunFactory{}, path, "api-core", "--", "printenv", "APP_NAME",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if stdout != "api-core\n" {
		t.Errorf("APP_NAME = %q, want api-core (file wins via ENVX_OVERLOAD)", stdout)
	}
}

// TestNewRunCommandOverloadFlag verifies --overload lets file values win over an
// OS env var.
func TestNewRunCommandOverloadFlag(t *testing.T) {
	t.Setenv("APP_NAME", "from-os")

	path := fixtures.Manifest("basic")
	stdout, _, err := executeRun(
		t, &mockRunFactory{}, path,
		"api-core", "--overload", "--", "printenv", "APP_NAME",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if stdout != "api-core\n" {
		t.Errorf("APP_NAME = %q, want api-core (file wins via --overload)", stdout)
	}
}

// TestNewRunCommandUnionsOSKeys verifies the child receives OS-only environment
// variables too, so the effective environment stays complete now that
// Materialize (not the runner) composes it.
func TestNewRunCommandUnionsOSKeys(t *testing.T) {
	t.Setenv("OS_ONLY_VAR", "present")

	path := fixtures.Manifest("basic")
	stdout, _, err := executeRun(
		t, &mockRunFactory{}, path, "api-core", "--", "printenv", "OS_ONLY_VAR",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if stdout != "present\n" {
		t.Errorf("child OS_ONLY_VAR = %q, want present", stdout)
	}
}

// TestNewRunCommandFactoryError verifies an environment service failure aborts the
// run before any child starts.
func TestNewRunCommandFactoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("factory failure")
	stdout, _, err := executeRun(
		t, &mockRunFactory{err: expectedErr}, "envx.yaml",
		"api-core", "--", "printenv", "APP_NAME",
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("got err = %v, want %v", err, expectedErr)
	}
	if stdout != "" {
		t.Errorf("child produced output %q despite the factory failure", stdout)
	}
}

// TestNewRunCommandRunnerFactoryError verifies a runner service failure aborts
// the run before any child starts.
func TestNewRunCommandRunnerFactoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("runner failure")
	stdout, _, err := executeRun(
		t, &mockRunFactory{runnerErr: expectedErr}, fixtures.Manifest("basic"),
		"api-core", "--", "printenv", "APP_NAME",
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("got err = %v, want %v", err, expectedErr)
	}
	if stdout != "" {
		t.Errorf("child produced output %q despite the factory failure", stdout)
	}
}

// TestNewRunCommandRevealFailurePreventsChildStartup verifies run reveals secrets
// before starting the child, so a reference it cannot decrypt fails during
// resolution and the child process never runs. The workspace stores real
// ciphertext but ships no private key, so the required key is unavailable.
func TestNewRunCommandRevealFailurePreventsChildStartup(t *testing.T) {
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
		"public-keys:\n  production: "+pair.PublicKey+
			"\nsecrets:\n  production:\n    db: "+ciphertext+"\n")

	stdout, _, err := executeRun(
		t, &mockRunFactory{}, filepath.Join(dir, "envx.yaml"),
		"api", "--", "printenv", "PASSWORD",
	)
	if err == nil {
		t.Fatal("expected the reveal failure to prevent child-process startup")
	}
	if stdout != "" {
		t.Errorf("child produced output %q despite the reveal failure", stdout)
	}
}

// TestNewRunCommandIgnoreErrorsFailsClosedByDefault verifies that without
// --ignore-errors an unresolved reference aborts the run before the child starts.
func TestNewRunCommandIgnoreErrorsFailsClosedByDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeWorkspaceFile(t, dir, "envx.yaml",
		"environments: [development]\nprojects:\n  api:\n    includes: [env/app]\n")
	writeWorkspaceFile(t, dir, filepath.Join("env", "app.yaml"),
		"good: value\nbroken: \"{{NOPE}}\"\n")

	stdout, _, err := executeRun(
		t, &mockRunFactory{}, filepath.Join(dir, "envx.yaml"),
		"api", "--", "printenv", "GOOD",
	)
	if err == nil {
		t.Fatal("expected the missing reference to abort the run")
	}
	if stdout != "" {
		t.Errorf("child produced output %q despite the failure", stdout)
	}
}

// TestNewRunCommandIgnoreErrorsStartsChild verifies --ignore-errors downgrades an
// unresolved reference to a stderr warning, omits its key (leaving it unset, not
// empty), and still starts the child with the keys that did resolve.
func TestNewRunCommandIgnoreErrorsStartsChild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeWorkspaceFile(t, dir, "envx.yaml",
		"environments: [development]\nprojects:\n  api:\n    includes: [env/app]\n")
	writeWorkspaceFile(t, dir, filepath.Join("env", "app.yaml"),
		"good: value\nbroken: \"{{NOPE}}\"\n")

	stdout, stderr, err := executeRun(
		t, &mockRunFactory{}, filepath.Join(dir, "envx.yaml"),
		"api", "--ignore-errors", "--",
		"sh", "-c", "echo GOOD=$GOOD; echo BROKEN=${BROKEN-<unset>}",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	// GOOD resolves; BROKEN is omitted entirely, so the child sees it as unset
	// rather than an empty string.
	if stdout != "GOOD=value\nBROKEN=<unset>\n" {
		t.Errorf("child output = %q, want GOOD=value + BROKEN unset", stdout)
	}
	if !strings.Contains(stderr, "WARNING") || !strings.Contains(stderr, "BROKEN") {
		t.Errorf("stderr = %q, want a warning naming BROKEN", stderr)
	}
}

// TestNewRunCommandIgnoreErrorsKeepsAmbientValue verifies that under --overload a
// broken file value which the shell already defines is not omitted: the ambient
// value survives so the file value never clobbers it.
func TestNewRunCommandIgnoreErrorsKeepsAmbientValue(t *testing.T) {
	t.Setenv("BROKEN", "from-shell")
	t.Setenv("ENVX_OVERLOAD", "true")

	dir := t.TempDir()
	writeWorkspaceFile(t, dir, "envx.yaml",
		"environments: [development]\nprojects:\n  api:\n    includes: [env/app]\n")
	writeWorkspaceFile(t, dir, filepath.Join("env", "app.yaml"),
		"broken: \"{{MISSING}}\"\n")

	stdout, stderr, err := executeRun(
		t, &mockRunFactory{}, filepath.Join(dir, "envx.yaml"),
		"api", "--ignore-errors", "--", "sh", "-c", "echo BROKEN=$BROKEN",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if stdout != "BROKEN=from-shell\n" {
		t.Errorf("child BROKEN = %q, want the ambient from-shell", stdout)
	}
	if !strings.Contains(stderr, "keeping the value") {
		t.Errorf("stderr = %q, want a fallback warning", stderr)
	}
}

// TestNewRunCommandArgsValidation verifies run's positional layout is enforced
// as a usage error.
func TestNewRunCommandArgsValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing separator", []string{"api"}, "missing '--' separator"},
		{"no project", []string{"--", "ls"}, "exactly one project"},
		{"two projects", []string{"a", "b", "--", "ls"}, "exactly one project"},
		{"no command", []string{"api", "--"}, "no command specified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := NewRunCommand(&mockRunFactory{})
			cmd.SetArgs(tt.args)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Execute() error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

// TestOutputRunWarnings verifies each downgraded failure is printed as a warning.
func TestOutputRunWarnings(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	console := printer.New(printer.Options{
		Out: &bytes.Buffer{},
		Err: &stderr,
	})

	err := outputRunWarnings(console, []error{
		errors.New("first problem"),
		errors.New("second problem"),
	})
	if err != nil {
		t.Fatalf("outputRunWarnings(): %v", err)
	}

	got := stderr.String()
	for _, want := range []string{"first problem", "second problem"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr missing %q:\n%s", want, got)
		}
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
