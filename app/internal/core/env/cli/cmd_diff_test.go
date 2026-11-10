package cli

import (
	"strings"
	"testing"
)

// TestDiffCommandOutput verifies the table and JSON renders against their
// goldens.
func TestDiffCommandOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// golden names the expected output file.
		golden string
		// args are the flags after the environments.
		args []string
	}{
		{golden: "diff.txt"},
		{golden: "diff.json", args: []string{"--output", "json"}},
	}
	for _, tt := range tests {
		t.Run(tt.golden, func(t *testing.T) {
			t.Parallel()

			args := append([]string{"app", "development", "production"}, tt.args...)
			stdout, _, err := execute(t, NewDiffCommand(newFakeFactory()), args...)
			if err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			assertGolden(t, tt.golden, stdout)
		})
	}
}

// TestDiffCommandRejectsEnvFlag verifies --env is not accepted, since both
// environments are positional.
func TestDiffCommandRejectsEnvFlag(t *testing.T) {
	t.Parallel()

	_, _, err := execute(
		t, NewDiffCommand(newFakeFactory()),
		"app", "development", "production", "--env", "production",
	)
	if err == nil || !strings.Contains(err.Error(), "unknown flag: --env") {
		t.Fatalf("err = %v, want an unknown --env flag error", err)
	}
}

// TestDiffCommandInvalidOutput verifies an unknown --output is rejected.
func TestDiffCommandInvalidOutput(t *testing.T) {
	t.Parallel()

	_, _, err := execute(
		t, NewDiffCommand(newFakeFactory()),
		"app", "development", "production", "--output", "yaml",
	)
	if err == nil || !strings.Contains(err.Error(), "invalid output format") {
		t.Fatalf("err = %v, want an invalid output format error", err)
	}
}
