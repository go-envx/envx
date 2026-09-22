package cli

import (
	"bytes"
	"strings"
	"testing"
)

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
