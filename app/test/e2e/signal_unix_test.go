//go:build e2e && !windows

package e2e_test

import (
	"bufio"
	"syscall"
	"testing"
)

// TestRunForwardsSignal verifies SIGTERM sent to envx reaches the child, whose
// resulting exit status envx mirrors.
func TestRunForwardsSignal(t *testing.T) {
	t.Parallel()

	cfg := basicWorkspace(t)
	cmd, _ := command(t, nil, append(
		[]string{"run", "--config", cfg, "api-core", "--"},
		helper(t, "await-term")...,
	)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// The child installs its handler before announcing readiness.
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "ready\n" {
		t.Fatalf("readiness line = %q, %v", line, err)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if code := cmd.ProcessState.ExitCode(); code != 42 {
		t.Errorf("exit %d, want the child's handled status 42", code)
	}
}
