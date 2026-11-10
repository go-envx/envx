package cli

import (
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
