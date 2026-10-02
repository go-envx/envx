package runner

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/shared/exitcode"
)

// TestRunDefaultsNilStreams verifies that nil streams fall back to the process's
// standard streams rather than failing.
func TestRunDefaultsNilStreams(t *testing.T) {
	t.Parallel()

	svc := NewService()
	err := svc.Run(RunParams{
		Args: []string{"sh", "-c", "exit 0"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// TestRunUsesInjectedStreams verifies the supplied stdin reaches the child and
// its stdout and stderr land in the supplied writers.
func TestRunUsesInjectedStreams(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	svc := NewService()
	err := svc.Run(RunParams{
		Args:   []string{"sh", "-c", "cat; echo err >&2"},
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader("input"),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := stdout.String(); got != "input" {
		t.Errorf("stdout = %q, want %q", got, "input")
	}
	if got := stderr.String(); got != "err\n" {
		t.Errorf("stderr = %q, want %q", got, "err\n")
	}
}

// TestRunInjectsEnv verifies the merged env is passed to the child and that only
// the supplied values appear in its environment.
func TestRunInjectsEnv(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	svc := NewService()
	err := svc.Run(RunParams{
		Args:   []string{"printenv", "FROM_FILE"},
		Env:    map[string]string{"FROM_FILE": "yes"},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := stdout.String(); got != "yes\n" {
		t.Errorf("child saw %q, want %q", got, "yes\n")
	}
}

// TestRunPropagatesExitCode verifies a non-zero child exit is surfaced as an
// *exitcode.Error carrying the same code.
func TestRunPropagatesExitCode(t *testing.T) {
	t.Parallel()

	svc := NewService()
	err := svc.Run(RunParams{
		Args:   []string{"sh", "-c", "exit 3"},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %v", err)
	}
	if ec.Code != 3 {
		t.Errorf("Code = %d, want 3", ec.Code)
	}
}

// TestRunNoCommand verifies an empty argument list returns ErrNoCommandSpecified.
func TestRunNoCommand(t *testing.T) {
	t.Parallel()

	svc := NewService()
	err := svc.Run(RunParams{})
	if !errors.Is(err, ErrNoCommandSpecified) {
		t.Fatalf("expected ErrNoCommandSpecified, got %v", err)
	}
}

// TestRunCommandNotFound verifies a command missing from PATH surfaces as an
// error wrapping ErrProcessStartFailed and an *exitcode.Error carrying code 127.
func TestRunCommandNotFound(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	svc := NewService()
	err := svc.Run(RunParams{
		Args:   []string{"envx-nonexistent-command-xyz"},
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
	})
	if !errors.Is(err, ErrProcessStartFailed) {
		t.Fatalf("expected ErrProcessStartFailed, got %v", err)
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %v", err)
	}
	if ec.Code != 127 {
		t.Errorf("Code = %d, want 127 (command not found)", ec.Code)
	}
	if stderr.Len() == 0 {
		t.Error("expected a diagnostic on stderr")
	}
}

// TestRunSignaledExitCode verifies a child terminated by a signal surfaces as an
// *exitcode.Error carrying the shell convention 128+signum (130 for SIGINT)
// rather than the -1 that os/exec reports for signaled processes.
func TestRunSignaledExitCode(t *testing.T) {
	t.Parallel()

	// The child signals only its own PID ($$), so the test process is unaffected.
	svc := NewService()
	err := svc.Run(RunParams{
		Args:   []string{"sh", "-c", "kill -INT $$"},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %v", err)
	}
	if ec.Code != 130 {
		t.Errorf("Code = %d, want 130 (128+SIGINT)", ec.Code)
	}
}
