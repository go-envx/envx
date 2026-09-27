package runner

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/shared/exitcode"
)

// TestNewServiceDefaultsStreams verifies that nil streams fall back to the process's
// standard streams.
func TestNewServiceDefaultsStreams(t *testing.T) {
	t.Parallel()

	svc := NewService(ServiceParams{})
	if svc.params.Stdout != os.Stdout {
		t.Errorf("Stdout = %v, want os.Stdout", svc.params.Stdout)
	}
	if svc.params.Stderr != os.Stderr {
		t.Errorf("Stderr = %v, want os.Stderr", svc.params.Stderr)
	}
	if svc.params.Stdin != os.Stdin {
		t.Errorf("Stdin = %v, want os.Stdin", svc.params.Stdin)
	}
}

// TestNewServicePreservesStreams verifies that explicit streams are left untouched.
func TestNewServicePreservesStreams(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	in := strings.NewReader("input")
	svc := NewService(ServiceParams{
		Stdout: &out,
		Stderr: &errBuf,
		Stdin:  in,
	})

	if svc.params.Stdout != &out {
		t.Error("Stdout was replaced, want the provided writer")
	}
	if svc.params.Stderr != &errBuf {
		t.Error("Stderr was replaced, want the provided writer")
	}
	if svc.params.Stdin != in {
		t.Error("Stdin was replaced, want the provided reader")
	}
}

// TestRunInjectsEnv verifies the merged env is passed to the child and that only
// the supplied values appear in its environment.
func TestRunInjectsEnv(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	svc := NewService(ServiceParams{
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
	})
	err := svc.Run(RunParams{
		Args: []string{"printenv", "FROM_FILE"},
		Env:  map[string]string{"FROM_FILE": "yes"},
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

	svc := NewService(ServiceParams{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	err := svc.Run(RunParams{
		Args: []string{"sh", "-c", "exit 3"},
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

	svc := NewService(ServiceParams{})
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
	svc := NewService(ServiceParams{
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
	})
	err := svc.Run(RunParams{
		Args: []string{"envx-nonexistent-command-xyz"},
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
	svc := NewService(ServiceParams{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	err := svc.Run(RunParams{
		Args: []string{"sh", "-c", "kill -INT $$"},
	})
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %v", err)
	}
	if ec.Code != 130 {
		t.Errorf("Code = %d, want 130 (128+SIGINT)", ec.Code)
	}
}
