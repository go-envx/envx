package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------
// TestRunCommand verifies end-to-end "envx run" behavior across multiple
// scenarios: namespace merging, invalid environments, invalid projects, and
// missing separators.
func TestRunCommand(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell")
	}

	fixtureDir := testdataDir(t, "basic")

	tests := []struct {
		name    string
		args    []string
		wantEnv map[string]string
		wantErr string
	}{
		{
			name: "api-core development merges all namespaces",
			args: []string{
				"run", "--config", filepath.Join(fixtureDir, "envx.yaml"),
				"--env", "development",
				"api-core", "--", "env",
			},
			wantEnv: map[string]string{
				// postgres.development overrides base
				"HOST": "dev-db.local",
				// api-core.development wins (last)
				"PORT": "3001",
				// from postgres base
				"CREDENTIALS_USERNAME": "postgres",
				// postgres.development overrides
				"CREDENTIALS_PASSWORD": "dev-secret",
				// gateway.development
				"URL": "http://localhost:8080",
				// gateway.development
				"TIMEOUT": "5",
				// api-core base
				"APP_NAME": "api-core",
				// api-core.development
				"LOG_LEVEL": "debug",
			},
		},
		{
			name: "web development merges gateway + web",
			args: []string{
				"run", "--config", filepath.Join(fixtureDir, "envx.yaml"),
				"--env", "development",
				"web", "--", "env",
			},
			wantEnv: map[string]string{
				// gateway.development
				"URL": "http://localhost:8080",
				// gateway.development
				"TIMEOUT": "5",
				// web base
				"APP_NAME": "web",
				// web.development wins
				"PORT": "4001",
			},
		},
		{
			name: "invalid environment",
			args: []string{
				"run", "--config", filepath.Join(fixtureDir, "envx.yaml"),
				"--env", "nonexistent",
				"api-core", "--", "env",
			},
			wantErr: "not declared in the manifest",
		},
		{
			name: "invalid project",
			args: []string{
				"run", "--config", filepath.Join(fixtureDir, "envx.yaml"),
				"--env", "development",
				"no-such-project", "--", "env",
			},
			wantErr: "not found in manifest",
		},
		{
			name: "missing -- separator",
			args: []string{
				"run",
				"--config",
				filepath.Join(fixtureDir, "envx.yaml"),
				"api-core",
			},
			wantErr: "usage:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout, _, err := execCmd(tt.args...)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Parse env output.
			envOutput := parseEnvOutput(stdout.String())
			for key, want := range tt.wantEnv {
				got, ok := envOutput[key]
				if !ok {
					t.Errorf("key %q not found in output", key)
					continue
				}
				if got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
		})
	}
}

// -------------------------------------------------------------------------------------
// TestRunCommandExitCode verifies that non-zero child exit codes are
// propagated as ExitCodeError.
func TestRunCommandExitCode(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell")
	}

	fixtureDir := testdataDir(t, "basic")

	_, _, err := execCmd(
		"run", "--config", filepath.Join(fixtureDir, "envx.yaml"),
		"--env", "development",
		"api-core", "--", "sh", "-c", "exit 7",
	)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}

	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 7 {
		t.Errorf("exit code = %d, want 7", exitErr.Code)
	}
}

// -------------------------------------------------------------------------------------
// TestRunStrictFlag verifies that --strict errors when environment overlay
// files are missing.
func TestRunStrictFlag(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell")
	}

	fixtureDir := testdataDir(t, "basic")

	// staging has no overlay files — strict mode should error.
	_, _, err := execCmd(
		"run", "--config", filepath.Join(fixtureDir, "envx.yaml"),
		"--env", "staging", "--strict",
		"api-core", "--", "env",
	)
	if err == nil {
		t.Fatal("expected error in strict mode for missing overlay")
	}
}

// -------------------------------------------------------------------------------------
// TestRunNamespacePrefixEnabled verifies that --namespace-prefix=true
// produces prefixed env var keys.
func TestRunNamespacePrefixEnabled(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell")
	}

	fixtureDir := testdataDir(t, "basic")

	stdout, _, err := execCmd(
		"run", "--config", filepath.Join(fixtureDir, "envx.yaml"),
		"--env", "development", "--namespace-prefix=true",
		"api-core", "--", "env",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envOutput := parseEnvOutput(stdout.String())
	// With namespace prefix, keys should have the namespace name.
	// e.g. "API_CORE_PORT" instead of "PORT"
	if _, ok := envOutput["API_CORE_PORT"]; !ok {
		t.Error("expected 'API_CORE_PORT' key when namespace-prefix is enabled")
	}
	if _, ok := envOutput["PORT"]; ok {
		t.Error("did not expect bare 'PORT' key when namespace-prefix is enabled")
	}
}

// -------------------------------------------------------------------------------------
// TestRunOverloadFlag verifies that --overload lets file values override
// existing OS environment variables.
func TestRunOverloadFlag(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell")
	}

	fixtureDir := testdataDir(t, "basic")

	// Set an env var that will conflict with a file value.
	t.Setenv("HOST", "os-set-value")

	// Without overload: OS env should win.
	stdout, _, err := execCmd(
		"run", "--config", filepath.Join(fixtureDir, "envx.yaml"),
		"--env", "development",
		"api-core", "--", "env",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envOutput := parseEnvOutput(stdout.String())
	if got := envOutput["HOST"]; got != "os-set-value" {
		t.Errorf("without --overload: HOST = %q, want %q", got, "os-set-value")
	}

	// With overload: file value should win.
	stdout, _, err = execCmd(
		"run", "--config", filepath.Join(fixtureDir, "envx.yaml"),
		"--env", "development", "--overload",
		"api-core", "--", "env",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envOutput = parseEnvOutput(stdout.String())
	if got := envOutput["HOST"]; got == "os-set-value" {
		t.Errorf(
			"with --overload: HOST should be file value, got OS value %q",
			got,
		)
	}
}

// -------------------------------------------------------------------------------------
// TestRunPrefixSuffixFlags verifies that --prefix and --suffix are applied
// to all env var keys in the output.
func TestRunPrefixSuffixFlags(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell")
	}

	fixtureDir := testdataDir(t, "basic")

	stdout, _, err := execCmd(
		"run", "--config", filepath.Join(fixtureDir, "envx.yaml"),
		"--env", "development",
		"--prefix", "MYAPP", "--suffix", "V2",
		"api-core", "--", "env",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envOutput := parseEnvOutput(stdout.String())
	// With prefix MYAPP and suffix V2, keys should be like MYAPP_HOST_V2
	if _, ok := envOutput["MYAPP_HOST_V2"]; !ok {
		t.Error("expected 'MYAPP_HOST_V2' key with --prefix and --suffix")
	}
}

// -------------------------------------------------------------------------------------
// execCmd creates a root command wired to fresh stdout/stderr buffers, sets the
// given args, and executes it. It returns the captured buffers and any error.
func execCmd(args ...string) (stdout, stderr *bytes.Buffer, err error) {
	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)
	cmd := NewRootCmd("test")
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return
}

// -------------------------------------------------------------------------------------
// testdataDir returns the absolute path to the specified testdata subdirectory.
// It looks for testdata relative to this test file.
func testdataDir(t *testing.T, name string) string {
	t.Helper()
	// Find testdata relative to this file.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", name)
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("testdata dir %q not found: %v", abs, err)
	}
	return abs
}

// -------------------------------------------------------------------------------------
// parseEnvOutput parses the output of the "env" command into a map of key-value pairs.
func parseEnvOutput(output string) map[string]string {
	result := make(map[string]string)
	for line := range strings.SplitSeq(output, "\n") {
		idx := strings.IndexByte(line, '=')
		if idx > 0 {
			result[line[:idx]] = line[idx+1:]
		}
	}
	return result
}
