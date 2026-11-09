package process

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-envx/envx/app/internal/lib/exitcode"
	"github.com/go-envx/envx/app/internal/lib/stdio"
)

func TestRunNoCommand(t *testing.T) {
	t.Parallel()

	err := Run(Command{})
	if !errors.Is(err, ErrNoCommand) {
		t.Fatalf("expected ErrNoCommand, got %v", err)
	}
}

func TestRunDefaultsNilStreams(t *testing.T) {
	t.Parallel()

	err := Run(Command{
		Args: []string{"sh", "-c", "exit 0"},
	})
	if err != nil {
		t.Fatalf("Run with nil streams: %v", err)
	}
}

func TestRunUsesInjectedStreams(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := Run(Command{
		Args: []string{"sh", "-c", "cat; echo err >&2"},
		Streams: stdio.Streams{
			In:     strings.NewReader("input data"),
			Out:    &stdout,
			ErrOut: &stderr,
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := stdout.String(); got != "input data" {
		t.Errorf("stdout = %q, want %q", got, "input data")
	}
	if got := stderr.String(); got != "err\n" {
		t.Errorf("stderr = %q, want %q", got, "err\n")
	}
}

func TestRunInjectsEnv(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := Run(Command{
		Args: []string{"sh", "-c", `echo "$FOO $BAR"`},
		Env: map[string]string{
			"FOO": "hello",
			"BAR": "world",
		},
		Streams: stdio.Streams{
			Out: &stdout,
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := stdout.String(); got != "hello world\n" {
		t.Errorf("stdout = %q, want %q", got, "hello world\n")
	}
}

func TestRunNilEnvDoesNotInheritAmbient(t *testing.T) {
	// Not parallel because t.Setenv mutates the process environment.
	ambientKey := "TEST_ENVX_AMBIENT_VAR"
	t.Setenv(ambientKey, "ambient_value")

	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "nil env",
			env:  nil,
		},
		{
			name: "empty env map",
			env:  map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			err := Run(Command{
				Args: []string{"sh", "-c", `echo "val=$` + ambientKey + `"`},
				Env:  tt.env,
				Streams: stdio.Streams{
					Out: &stdout,
				},
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if got := stdout.String(); got != "val=\n" {
				t.Errorf("child inherited ambient var: got %q, want %q", got, "val=\n")
			}
		})
	}
}

func TestRunPropagatesExitCode(t *testing.T) {
	t.Parallel()

	err := Run(Command{
		Args: []string{"sh", "-c", "exit 42"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	exitErr, ok := errors.AsType[*exitcode.Error](err)
	if !ok {
		t.Fatalf("expected *exitcode.Error, got %T (%v)", err, err)
	}
	if exitErr.Code != 42 {
		t.Errorf("Code = %d, want 42", exitErr.Code)
	}
	if got := exitErr.Error(); got != "exit status 42" {
		t.Errorf("Error() = %q, want %q", got, "exit status 42")
	}
}

func TestRunCommandNotFound(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	err := Run(Command{
		Args: []string{"envx-nonexistent-command-xyz-12345"},
		Streams: stdio.Streams{
			ErrOut: &stderr,
		},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	exitErr, ok := errors.AsType[*exitcode.Error](err)
	if !ok {
		t.Fatalf("expected *exitcode.Error, got %T (%v)", err, err)
	}
	if exitErr.Code != 127 {
		t.Errorf("Code = %d, want 127", exitErr.Code)
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("expected cause to be exec.ErrNotFound, got %v", exitErr.Cause)
	}
	// process runner does not print application diagnostics
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunCommandNotExecutable(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not_executable.sh")
	if err := os.WriteFile(filePath, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := Run(Command{
		Args: []string{filePath},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	exitErr, ok := errors.AsType[*exitcode.Error](err)
	if !ok {
		t.Fatalf("expected *exitcode.Error, got %T (%v)", err, err)
	}
	if exitErr.Code != 126 {
		t.Errorf("Code = %d, want 126", exitErr.Code)
	}
}

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

func TestMapToEnv(t *testing.T) {
	t.Parallel()

	t.Run("nil map returns non-nil empty slice", func(t *testing.T) {
		t.Parallel()
		got := mapToEnv(nil)
		if got == nil {
			t.Fatal("expected non-nil slice, got nil")
		}
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("empty map returns non-nil empty slice", func(t *testing.T) {
		t.Parallel()
		got := mapToEnv(map[string]string{})
		if got == nil {
			t.Fatal("expected non-nil slice, got nil")
		}
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("populated map returns sorted KEY=VALUE entries", func(t *testing.T) {
		t.Parallel()
		envMap := map[string]string{
			"ZEBRA": "last",
			"APPLE": "first",
			"MANGO": "middle",
		}
		got := mapToEnv(envMap)
		want := []string{"APPLE=first", "MANGO=middle", "ZEBRA=last"}
		if !slices.Equal(got, want) {
			t.Errorf("mapToEnv = %v, want %v", got, want)
		}
	})
}

func TestStartFailureCode(t *testing.T) {
	t.Parallel()

	if code := startFailureCode(exec.ErrNotFound); code != 127 {
		t.Errorf("startFailureCode(ErrNotFound) = %d, want 127", code)
	}
	if code := startFailureCode(os.ErrNotExist); code != 127 {
		t.Errorf("startFailureCode(ErrNotExist) = %d, want 127", code)
	}
	if code := startFailureCode(os.ErrPermission); code != 126 {
		t.Errorf("startFailureCode(ErrPermission) = %d, want 126", code)
	}
	if code := startFailureCode(errors.New("other start error")); code != 126 {
		t.Errorf("startFailureCode(other) = %d, want 126", code)
	}
}

func TestStdinIsTerminal(t *testing.T) {
	t.Parallel()

	t.Run("non-file reader", func(t *testing.T) {
		t.Parallel()
		if got := stdinIsTerminal(strings.NewReader("")); got {
			t.Error("stdinIsTerminal(strings.Reader) = true, want false")
		}
	})

	t.Run("pipe file", func(t *testing.T) {
		t.Parallel()
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("Pipe: %v", err)
		}
		defer func() { _ = r.Close() }()
		defer func() { _ = w.Close() }()

		if got := stdinIsTerminal(r); got {
			t.Error("stdinIsTerminal(pipe) = true, want false")
		}
	})

	t.Run("nil reader", func(t *testing.T) {
		t.Parallel()
		if got := stdinIsTerminal(nil); got {
			t.Error("stdinIsTerminal(nil) = true, want false")
		}
	})
}
