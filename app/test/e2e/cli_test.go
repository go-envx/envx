//go:build e2e

package e2e_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRootShowsHelp verifies bare invocation prints help and does not require a
// manifest.
func TestRootShowsHelp(t *testing.T) {
	t.Parallel()

	res := mustEnvx(t, nil)
	if !strings.Contains(res.stdout, "envx") {
		t.Errorf("expected help output, got %q", res.stdout)
	}
}

// TestKeypairGenerateRequiresGroup verifies persisted generation rejects a
// missing group before trying to resolve a workspace.
func TestKeypairGenerateRequiresGroup(t *testing.T) {
	t.Parallel()

	res := envx(t, nil, "keypair", "generate")
	if res.code != 2 || !strings.Contains(res.stderr, "accepts 1 arg") {
		t.Fatalf(
			"exit %d, stderr %q; want usage exit 2 with missing-group error",
			res.code,
			res.stderr,
		)
	}
}

// TestVersionFlag verifies --version prints the injected version.
func TestVersionFlag(t *testing.T) {
	t.Parallel()

	res := mustEnvx(t, nil, "--version")
	if !strings.HasPrefix(res.stdout, "envx version test\n") {
		t.Errorf(
			"version output = %q, want it to start with %q",
			res.stdout,
			"envx version test\n",
		)
	}
	if !strings.Contains(res.stdout, "commit:") {
		t.Errorf("version output %q missing commit metadata", res.stdout)
	}
}

// TestGet verifies get prints a resolved value and reports failures with the
// matching exit code on stderr.
func TestGet(t *testing.T) {
	t.Parallel()

	cfg := basicWorkspace(t)

	tests := []struct {
		name     string
		args     []string
		want     string
		wantCode int
		wantErr  string
	}{
		{
			name: "overlay value",
			args: []string{
				"get", "--config", cfg, "--env", "development", "api-core", "HOST",
			},
			want: "dev-db.local",
		},
		{
			name: "later namespace overrides",
			args: []string{
				"get", "--config", cfg, "--env", "development", "api-core", "PORT",
			},
			want: "3001",
		},
		{
			name: "missing key",
			args: []string{
				"get", "--config", cfg, "--env", "development", "api-core", "NOPE",
			},
			wantCode: 1,
			wantErr:  "not found",
		},
		{
			name: "unknown project",
			args: []string{
				"get", "--config", cfg, "--env", "development", "ghost", "HOST",
			},
			wantCode: 1,
			wantErr:  "not found in manifest",
		},
		{
			name:     "bad arg count",
			args:     []string{"get", "--config", cfg, "api-core"},
			wantCode: 2,
			wantErr:  "accepts 2 arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := envx(t, nil, tt.args...)
			if res.code != tt.wantCode {
				t.Fatalf(
					"exit %d, want %d\nstderr:\n%s",
					res.code,
					tt.wantCode,
					res.stderr,
				)
			}
			if tt.wantErr != "" {
				if !strings.Contains(res.stderr, tt.wantErr) {
					t.Errorf("stderr = %q, want containing %q", res.stderr, tt.wantErr)
				}
				return
			}
			if got := strings.TrimSpace(res.stdout); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRun verifies run injects the merged environment into the child process.
func TestRun(t *testing.T) {
	t.Parallel()

	cfg := basicWorkspace(t)
	res := mustEnvx(t, nil,
		"run", "--config", cfg, "--env", "development", "--overload",
		"api-core", "--", "printenv", "APP_NAME",
	)
	if got := strings.TrimSpace(res.stdout); got != "api-core" {
		t.Errorf("child APP_NAME = %q, want api-core", got)
	}
}

// TestRunRequiresCommand verifies run rejects a missing "-- command".
func TestRunRequiresCommand(t *testing.T) {
	t.Parallel()

	cfg := basicWorkspace(t)
	if res := envx(t, nil, "run", "--config", cfg, "api-core"); res.code == 0 {
		t.Fatal("expected failure when no command follows --")
	}
}

// TestEmitDotenv verifies emit renders a resolved environment as dotenv through
// the ordinary --config pipeline.
func TestEmitDotenv(t *testing.T) {
	t.Parallel()

	cfg := basicWorkspace(t)
	res := mustEnvx(t, nil,
		"emit", "--config", cfg, "--env", "development", "api-core",
		"--target", "dotenv",
	)
	if !strings.Contains(res.stdout, "APP_NAME=api-core\n") {
		t.Errorf("emit dotenv missing APP_NAME:\n%s", res.stdout)
	}
}

// TestEmitRequiresTarget verifies emit rejects an invocation with no --target.
func TestEmitRequiresTarget(t *testing.T) {
	t.Parallel()

	cfg := basicWorkspace(t)
	if res := envx(t, nil, "emit", "--config", cfg, "api-core"); res.code == 0 {
		t.Fatal("expected emit to require --target")
	}
}

// TestEmitRejectsNameForPlainTarget verifies --name is rejected for a target
// that does not render a Kubernetes resource, so it is never silently ignored.
func TestEmitRejectsNameForPlainTarget(t *testing.T) {
	t.Parallel()

	cfg := basicWorkspace(t)
	res := envx(t, nil,
		"emit", "--config", cfg, "api-core", "--target", "dotenv", "--name", "oops",
	)
	if res.code == 0 {
		t.Fatal("expected --name to be rejected for the dotenv target")
	}
}

// TestEmitK8sAutoNamesFromProject verifies a k8s target with no --name derives
// the resource name from the project (here api-core-config).
func TestEmitK8sAutoNamesFromProject(t *testing.T) {
	t.Parallel()

	cfg := basicWorkspace(t)
	res := mustEnvx(t, nil,
		"emit", "--config", cfg, "api-core", "--target", "k8s", "--only", "config",
	)
	if !strings.Contains(res.stdout, "name: api-core-config") {
		t.Errorf("expected auto-named ConfigMap api-core-config:\n%s", res.stdout)
	}
}

// TestExplain verifies explain prints values by default and supports JSON output.
func TestExplain(t *testing.T) {
	t.Parallel()

	cfg := basicWorkspace(t)

	t.Run("shows value", func(t *testing.T) {
		t.Parallel()
		res := mustEnvx(t, nil,
			"explain", "--config", cfg, "--env", "development", "api-core", "HOST",
		)
		if !strings.Contains(res.stdout, "HOST") {
			t.Errorf("expected HOST in output, got %q", res.stdout)
		}
		if !strings.Contains(res.stdout, "dev-db.local") {
			t.Errorf("expected value in output, got %q", res.stdout)
		}
	})

	t.Run("json output", func(t *testing.T) {
		t.Parallel()
		res := mustEnvx(t, nil,
			"explain", "--config", cfg, "--env", "development",
			"--output", "json", "api-core", "HOST",
		)
		var result struct {
			Entries []map[string]any `json:"entries"`
		}
		if err := json.Unmarshal([]byte(res.stdout), &result); err != nil {
			t.Fatalf("invalid json: %v\n%s", err, res.stdout)
		}
		if len(result.Entries) != 1 || result.Entries[0]["key"] != "HOST" {
			t.Errorf("unexpected json entries: %v", result.Entries)
		}
	})
}

// TestDiff verifies diff reports changed keys across two environments and prints
// their values by default — locking that diff both registers and reads the
// display flags.
func TestDiff(t *testing.T) {
	t.Parallel()

	cfg := basicWorkspace(t)
	res := mustEnvx(t, nil,
		"diff", "--config", cfg, "--output", "json",
		"api-core", "development", "production",
	)
	var result struct {
		Changed []map[string]string `json:"changed"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &result); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, res.stdout)
	}
	for _, c := range result.Changed {
		if c["key"] == "HOST" {
			if c["env_a"] != "dev-db.local" {
				t.Errorf("env_a = %q, want dev-db.local", c["env_a"])
			}
			return
		}
	}
	t.Fatalf("expected HOST among changed keys, got %v", result.Changed)
}
