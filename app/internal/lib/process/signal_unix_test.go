//go:build !windows

package process

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/go-envx/envx/app/internal/lib/exitcode"
	"github.com/go-envx/envx/app/internal/lib/stdio"
)

func TestRunSignaledExitCode(t *testing.T) {
	t.Parallel()

	// Child signals its own PID ($$) so the parent process is unaffected.
	err := Run(Command{
		Args: []string{"sh", "-c", "kill -INT $$"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	exitErr, ok := errors.AsType[*exitcode.Error](err)
	if !ok {
		t.Fatalf("expected *exitcode.Error, got %T (%v)", err, err)
	}
	if exitErr.Code != 130 {
		t.Errorf("Code = %d, want 130 (128 + SIGINT)", exitErr.Code)
	}
}

const signalHelperEnv = "ENVX_PROCESS_SIGNAL_HELPER"

// TestRunForwardsSignalToChild re-executes the test binary as the supervising
// parent so the signal can target it without affecting the test process.
func TestRunForwardsSignalToChild(t *testing.T) {
	if os.Getenv(signalHelperEnv) == "1" {
		err := Run(Command{
			Args:    []string{"sh", "-c", "echo ready; exec sleep 30"},
			Env:     map[string]string{"PATH": os.Getenv("PATH")},
			Streams: stdio.Streams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		})
		if exitErr, ok := errors.AsType[*exitcode.Error](err); ok {
			os.Exit(exitErr.Code)
		}
		os.Exit(0)
	}
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	//nolint:gosec // re-executes the test binary itself
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunForwardsSignalToChild$")
	cmd.Env = []string{signalHelperEnv + "=1", "PATH=" + os.Getenv("PATH")}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "ready\n" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("waiting for child readiness: line=%q err=%v", line, err)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Signal: %v", err)
	}

	err = cmd.Wait()
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T (%v)", err, err)
	}
	if got := exitErr.ExitCode(); got != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM)", got)
	}
}

func TestShouldForward(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		sig         os.Signal
		interactive bool
		want        bool
	}{
		{"interactive SIGINT not forwarded", os.Interrupt, true, false},
		{"interactive SIGQUIT not forwarded", syscall.SIGQUIT, true, false},
		{"interactive SIGTERM forwarded", syscall.SIGTERM, true, true},
		{"interactive SIGHUP forwarded", syscall.SIGHUP, true, true},
		{"non-interactive SIGINT forwarded", os.Interrupt, false, true},
		{"non-interactive SIGQUIT forwarded", syscall.SIGQUIT, false, true},
		{"non-interactive SIGTERM forwarded", syscall.SIGTERM, false, true},
		{"non-interactive SIGHUP forwarded", syscall.SIGHUP, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldForward(tt.sig, tt.interactive); got != tt.want {
				t.Errorf("shouldForward(%v, interactive=%v) = %v, want %v",
					tt.sig, tt.interactive, got, tt.want)
			}
		})
	}
}
