package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExplainCommandOutput verifies the table and JSON renders against their
// goldens.
func TestExplainCommandOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// golden names the expected output file.
		golden string
		// args are the flags after the project.
		args []string
	}{
		{golden: "explain.txt"},
		{golden: "explain.json", args: []string{"--output", "json"}},
	}
	for _, tt := range tests {
		t.Run(tt.golden, func(t *testing.T) {
			t.Parallel()

			args := append([]string{"app", "--env", "development"}, tt.args...)
			stdout, _, err := execute(t, NewExplainCommand(newFakeFactory()), args...)
			if err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			assertGolden(t, tt.golden, stdout)
		})
	}
}

// TestExplainCommandFlags verifies the key argument narrows the output and
// --reveal and --absolute change the rendered columns.
func TestExplainCommandFlags(t *testing.T) {
	t.Parallel()

	stdout, _, err := execute(
		t, NewExplainCommand(newFakeFactory()),
		"app", "host", "--env", "development", "--reveal", "--absolute",
	)
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	for _, want := range []string{"RESOLVED", appInclude + ".development.yaml"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "PORT") {
		t.Errorf("stdout lists an unrequested key:\n%s", stdout)
	}
}

// TestExplainCommandLocalEnvSource verifies a local env file winner is shown
// with its workspace-relative path in the table and JSON renders.
func TestExplainCommandLocalEnvSource(t *testing.T) {
	t.Parallel()

	for _, output := range []string{"table", "json"} {
		t.Run(output, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			file := filepath.Join(root, "envx.local.env")
			if err := os.WriteFile(file, []byte("HOST=from-file\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			factory := newFakeFactory()
			factory.root, factory.hostFile = root, file

			stdout, _, err := execute(
				t, NewExplainCommand(factory),
				"app", "host", "--env", "development", "--output", output,
			)
			if err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			if !strings.Contains(stdout, "envx.local.env") ||
				strings.Contains(stdout, root) {
				t.Errorf("stdout = %s, want relative envx.local.env source", stdout)
			}
			if output == "json" && !strings.Contains(stdout, `"sourceKind": "localEnv"`) {
				t.Errorf("stdout = %s, want sourceKind localEnv", stdout)
			}
		})
	}
}

// TestExplainCommandProcessSource verifies a process variable winner is
// labeled in the table and typed in the JSON.
func TestExplainCommandProcessSource(t *testing.T) {
	t.Parallel()

	tests := []struct{ output, want string }{
		{"table", "(process)"},
		{"json", `"sourceKind": "process"`},
	}
	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			t.Parallel()

			factory := newFakeFactory()
			factory.host = []string{"HOST=from-host"}
			stdout, _, err := execute(
				t, NewExplainCommand(factory),
				"app", "host", "--env", "development", "--output", tt.output,
			)
			if err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			if !strings.Contains(stdout, tt.want) {
				t.Errorf("stdout missing %q:\n%s", tt.want, stdout)
			}
		})
	}
}

// TestExplainCommandInvalidOutput verifies an unknown --output is rejected.
func TestExplainCommandInvalidOutput(t *testing.T) {
	t.Parallel()

	_, _, err := execute(
		t, NewExplainCommand(newFakeFactory()), "app", "--output", "yaml",
	)
	if err == nil || !strings.Contains(err.Error(), "invalid output format") {
		t.Fatalf("err = %v, want an invalid output format error", err)
	}
}
