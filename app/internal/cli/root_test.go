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

// TestExplain verifies masking by default, --reveal, and JSON output.
func TestExplain(t *testing.T) {
	t.Parallel()

	cfg := fixtures.Manifest("basic")

	t.Run("masks by default", func(t *testing.T) {
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
		if strings.Contains(out, "dev-db.local") {
			t.Errorf("expected value to be masked, got %q", out)
		}
	})

	t.Run("reveal shows value", func(t *testing.T) {
		t.Parallel()
		stdout, _, err := execCmd(
			"explain", "--config", cfg, "--env", "development",
			"--reveal", "api-core", "HOST",
		)
		if err != nil {
			t.Fatalf("explain --reveal: %v", err)
		}
		if !strings.Contains(stdout.String(), "dev-db.local") {
			t.Errorf("expected revealed value, got %q", stdout.String())
		}
	})

	t.Run("json output", func(t *testing.T) {
		t.Parallel()
		stdout, _, err := execCmd(
			"explain", "--config", cfg, "--env", "development",
			"--reveal", "--output", "json", "api-core", "HOST",
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

// TestDiff verifies diff reports changed keys across two environments and honors
// --reveal (values shown) versus the default (values masked) — locking that diff
// both registers and reads the display flags.
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

	t.Run("reveal shows values", func(t *testing.T) {
		t.Parallel()
		if got := changedHost(t, "--reveal"); got != "dev-db.local" {
			t.Errorf("env_a = %q, want dev-db.local (revealed)", got)
		}
	})
	t.Run("masks by default", func(t *testing.T) {
		t.Parallel()
		if got := changedHost(t); got != "********" {
			t.Errorf("env_a = %q, want masked", got)
		}
	})
}
