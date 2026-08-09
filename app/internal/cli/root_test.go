package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/fixtures"
	"github.com/go-envx/envx/app/pkg/file"
)

// -------------------------------------------------------------------------------------

// execCmd builds a root command wired to fresh stdout/stderr buffers, sets the
// given args, and executes it. It returns the captured buffers and any error.
func execCmd(args ...string) (stdout, stderr *bytes.Buffer, err error) {
	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)
	cmd := NewRootCmd(BuildInfo{Version: "test"})
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

// -------------------------------------------------------------------------------------

// copyTree recursively copies the directory tree at src into dst.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyTree(t, s, d)
			continue
		}
		data, err := file.Read(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.WriteAtomic(d, data); err != nil {
			t.Fatal(err)
		}
	}
}

// -------------------------------------------------------------------------------------

// TestRootShowsHelp verifies bare invocation prints help and does not require a
// manifest.
func TestRootShowsHelp(t *testing.T) {
	t.Parallel()

	stdout, _, err := execCmd()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "envx") {
		t.Errorf("expected help output, got %q", stdout.String())
	}
}

// -------------------------------------------------------------------------------------

// TestKeypairCommandTree verifies keypair management is registered at the root
// and no longer appears under the secrets command.
func TestKeypairCommandTree(t *testing.T) {
	t.Parallel()

	root := NewRootCmd(BuildInfo{Version: "test"})
	command, _, err := root.Find([]string{
		"keypair", "inspect",
	})
	if err != nil {
		t.Fatalf("Find(): %v", err)
	}
	if command == nil || command.Use != "inspect <group>" {
		t.Fatalf("command = %v, want inspect <group>", command)
	}
	command, _, err = root.Find([]string{"keypair", "print"})
	if err != nil {
		t.Fatalf("Find(print): %v", err)
	}
	if command == nil || command.Use != "print" {
		t.Fatalf("command = %v, want print", command)
	}
	secretsCommand, _, err := root.Find([]string{"secrets"})
	if err != nil {
		t.Fatalf("Find(secrets): %v", err)
	}
	for _, child := range secretsCommand.Commands() {
		if child.Name() == "keypair" {
			t.Fatal("secrets command still contains nested keypair")
		}
	}
}

// -------------------------------------------------------------------------------------

// TestKeypairPrintUsesConfiguredCipher verifies print honors the manifest
// cipher, leaves stderr empty, and does not create managed files.
func TestKeypairPrintUsesConfiguredCipher(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "envx.yaml")
	body := "environments: [production]\n" +
		"secrets:\n  cipher: nacl-box\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := execCmd(
		"keypair", "print", "--config", configPath,
	)
	if err != nil {
		t.Fatalf("keypair print: %v", err)
	}
	if !strings.Contains(stdout.String(), "nacl-box-public-key:") ||
		!strings.Contains(stdout.String(), "nacl-box-private-key:") {
		t.Errorf("stdout = %q, want configured NaCl Box keys", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
	for _, path := range []string{
		filepath.Join(dir, "secrets.yaml"),
		filepath.Join(dir, "envx.keys"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("stdout command created %s, stat error = %v", path, err)
		}
	}
}

// -------------------------------------------------------------------------------------

// TestKeypairPrintCipherFlagOverridesManifest verifies an explicit cipher flag
// wins over the algorithm configured in envx.yaml.
func TestKeypairPrintCipherFlagOverridesManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "envx.yaml")
	body := "environments: [production]\n" +
		"secrets:\n  cipher: age\n" +
		"projects:\n  app:\n    includes: [env/app]\n"
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := execCmd(
		"keypair", "print", "--config", configPath, "--cipher", "nacl-box",
	)
	if err != nil {
		t.Fatalf("keypair print --cipher: %v", err)
	}
	if !strings.Contains(stdout.String(), "nacl-box-public-key:") ||
		!strings.Contains(stdout.String(), "nacl-box-private-key:") {
		t.Errorf("stdout = %q, want flag-selected NaCl Box keys", stdout.String())
	}
}

// -------------------------------------------------------------------------------------

// TestKeypairGenerateRequiresGroup verifies persisted generation rejects a
// missing group before trying to resolve a workspace.
func TestKeypairGenerateRequiresGroup(t *testing.T) {
	t.Parallel()

	_, _, err := execCmd("keypair", "generate")
	if err == nil || !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Fatalf("error = %v, want missing-group validation error", err)
	}
}

// -------------------------------------------------------------------------------------

// TestVersionFlag verifies --version prints the injected version.
func TestVersionFlag(t *testing.T) {
	t.Parallel()

	stdout, _, err := execCmd("--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.HasPrefix(out, "envx version test\n") {
		t.Errorf("version output = %q, want it to start with %q", out, "envx version test\n")
	}
	if !strings.Contains(out, "commit:") {
		t.Errorf("version output %q missing commit metadata", out)
	}
}

// -------------------------------------------------------------------------------------

// TestGet verifies get prints a resolved value and reports missing keys.
func TestGet(t *testing.T) {
	t.Parallel()

	cfg := fixtures.Manifest("basic")

	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr string
	}{
		{
			name: "overlay value",
			args: []string{"get", "--config", cfg, "--env", "development", "api-core", "HOST"},
			want: "dev-db.local",
		},
		{
			name: "later namespace overrides",
			args: []string{"get", "--config", cfg, "--env", "development", "api-core", "PORT"},
			want: "3001",
		},
		{
			name: "missing key",
			args: []string{
				"get", "--config", cfg, "--env", "development", "api-core", "NOPE",
			},
			wantErr: "not found",
		},
		{
			name:    "unknown project",
			args:    []string{"get", "--config", cfg, "--env", "development", "ghost", "HOST"},
			wantErr: "not found in manifest",
		},
		{
			name:    "bad arg count",
			args:    []string{"get", "--config", cfg, "api-core"},
			wantErr: "accepts 2 arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout, _, err := execCmd(tt.args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := strings.TrimSpace(stdout.String()); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------------------

// TestRun verifies run injects the merged environment into the child process.
func TestRun(t *testing.T) {
	t.Parallel()

	cfg := fixtures.Manifest("basic")
	stdout, _, err := execCmd(
		"run", "--config", cfg, "--env", "development", "--overload",
		"api-core", "--", "printenv", "APP_NAME",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "api-core" {
		t.Errorf("child APP_NAME = %q, want api-core", got)
	}
}

// -------------------------------------------------------------------------------------

// TestRunRequiresCommand verifies run rejects a missing "-- command".
func TestRunRequiresCommand(t *testing.T) {
	t.Parallel()

	cfg := fixtures.Manifest("basic")
	if _, _, err := execCmd("run", "--config", cfg, "api-core"); err == nil {
		t.Fatal("expected error when no command follows --")
	}
}

// -------------------------------------------------------------------------------------

// TestSetRoundTrip verifies set writes a value that get can read back.
func TestSetRoundTrip(t *testing.T) {
	t.Parallel()

	work := t.TempDir()
	copyTree(t, fixtures.Testdata("basic"), work)
	cfg := filepath.Join(work, "envx.yaml")

	if _, _, err := execCmd(
		"set", "--config", cfg, "--env", "development",
		"env/postgres", "feature.enabled", "true",
	); err != nil {
		t.Fatalf("set: %v", err)
	}

	stdout, _, err := execCmd(
		"get", "--config", cfg, "--env", "development",
		"api-core", "FEATURE_ENABLED",
	)
	if err != nil {
		t.Fatalf("get after set: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "true" {
		t.Errorf("FEATURE_ENABLED = %q, want true", got)
	}
}

// -------------------------------------------------------------------------------------

// TestExplain verifies explain prints values by default and supports JSON output.
func TestExplain(t *testing.T) {
	t.Parallel()

	cfg := fixtures.Manifest("basic")

	t.Run("shows value", func(t *testing.T) {
		t.Parallel()
		stdout, _, err := execCmd(
			"explain", "--config", cfg, "--env", "development", "api-core", "HOST",
		)
		if err != nil {
			t.Fatalf("explain: %v", err)
		}
		out := stdout.String()
		if !strings.Contains(out, "HOST") {
			t.Errorf("expected HOST in output, got %q", out)
		}
		if !strings.Contains(out, "dev-db.local") {
			t.Errorf("expected value in output, got %q", out)
		}
	})

	t.Run("json output", func(t *testing.T) {
		t.Parallel()
		stdout, _, err := execCmd(
			"explain", "--config", cfg, "--env", "development",
			"--output", "json", "api-core", "HOST",
		)
		if err != nil {
			t.Fatalf("explain --output json: %v", err)
		}
		var entries []map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
			t.Fatalf("invalid json: %v\n%s", err, stdout.String())
		}
		if len(entries) != 1 || entries[0]["key"] != "HOST" {
			t.Errorf("unexpected json entries: %v", entries)
		}
	})
}

// -------------------------------------------------------------------------------------

// TestDiff verifies diff reports changed keys across two environments and prints
// their values by default — locking that diff both registers and reads the
// display flags.
func TestDiff(t *testing.T) {
	t.Parallel()

	cfg := fixtures.Manifest("basic")

	// changedHost runs diff for api-core development->production and returns the
	// env_a value of the HOST change, which differs between the two environments.
	changedHost := func(t *testing.T, args ...string) string {
		t.Helper()
		full := append([]string{"diff", "--config", cfg, "--output", "json"}, args...)
		full = append(full, "api-core", "development", "production")
		stdout, _, err := execCmd(full...)
		if err != nil {
			t.Fatalf("diff: %v", err)
		}
		var result struct {
			Changed []map[string]string `json:"changed"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("invalid json: %v\n%s", err, stdout.String())
		}
		for _, c := range result.Changed {
			if c["key"] == "HOST" {
				return c["env_a"]
			}
		}
		t.Fatalf("expected HOST among changed keys, got %v", result.Changed)
		return ""
	}

	t.Run("shows values", func(t *testing.T) {
		t.Parallel()
		if got := changedHost(t); got != "dev-db.local" {
			t.Errorf("env_a = %q, want dev-db.local", got)
		}
	})
}
