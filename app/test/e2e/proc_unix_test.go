//go:build e2e && !windows

package e2e_test

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// baseEnv is the minimal environment every invocation starts from.
func baseEnv(home string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
	}
}

// isolate starts the command as the leader of a new process group.
func isolate(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree kills the process group led by pid, including children that
// outlived the leader.
func killTree(pid int) error {
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

// cleanupTree kills whatever remains of a started command's process group.
func cleanupTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = killTree(cmd.Process.Pid)
	}
}
