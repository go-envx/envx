//go:build e2e

package e2e_test

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// baseEnv is the minimal environment every invocation starts from; Windows
// programs also expect SystemRoot and a writable TEMP.
func baseEnv(home string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"SystemRoot=" + os.Getenv("SystemRoot"),
		"USERPROFILE=" + home,
		"TEMP=" + os.TempDir(),
		"TMP=" + os.TempDir(),
	}
}

// isolate starts the command in a new process group so console events sent to
// the test do not reach it.
func isolate(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// killTree kills pid and its descendants.
func killTree(pid int) error {
	//nolint:gosec // fixed system tool; the PID comes from our own child.
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}

// cleanupTree kills a started command that was never waited for. An exited
// command is skipped because its PID may already belong to another process.
func cleanupTree(cmd *exec.Cmd) {
	if cmd.Process != nil && cmd.ProcessState == nil {
		_ = killTree(cmd.Process.Pid)
	}
}
