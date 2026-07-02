package runner

import (
	"bytes"
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/exitcode"
)

// -------------------------------------------------------------------------------------

// TestRunInjectsEnv verifies the merged env is passed to the child and that file
// values appear in its environment.
func TestRunInjectsEnv(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := Run([]string{"printenv", "FROM_FILE"}, Params{
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

// -------------------------------------------------------------------------------------

// TestRunPropagatesExitCode verifies a non-zero child exit is surfaced as an
// *exitcode.Error carrying the same code.
func TestRunPropagatesExitCode(t *testing.T) {
	t.Parallel()

	err := Run([]string{"sh", "-c", "exit 3"}, Params{
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

// -------------------------------------------------------------------------------------

// TestRunOverloadPrecedence verifies that Overload lets file values win over an
// OS env var of the same name.
func TestRunOverloadPrecedence(t *testing.T) {
	t.Setenv("SHARED_KEY", "from-os")

	var stdout bytes.Buffer
	err := Run([]string{"printenv", "SHARED_KEY"}, Params{
		Env:      map[string]string{"SHARED_KEY": "from-file"},
		Overload: true,
		Stdout:   &stdout,
		Stderr:   &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := stdout.String(); got != "from-file\n" {
		t.Errorf("with overload child saw %q, want %q", got, "from-file\n")
	}
}

// -------------------------------------------------------------------------------------

// TestRunDefaultPrecedence verifies that without Overload the OS env var wins.
func TestRunDefaultPrecedence(t *testing.T) {
	t.Setenv("SHARED_KEY", "from-os")

	var stdout bytes.Buffer
	err := Run([]string{"printenv", "SHARED_KEY"}, Params{
		Env:    map[string]string{"SHARED_KEY": "from-file"},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := stdout.String(); got != "from-os\n" {
		t.Errorf("without overload child saw %q, want %q", got, "from-os\n")
	}
}

// -------------------------------------------------------------------------------------

// TestRunNoCommand verifies an empty argument list is rejected.
func TestRunNoCommand(t *testing.T) {
	t.Parallel()

	if err := Run(nil, Params{}); err == nil {
		t.Fatal("expected error for empty args")
	}
}

// -------------------------------------------------------------------------------------

// TestRunSignaledExitCode verifies a child terminated by a signal surfaces as an
// *exitcode.Error carrying the shell convention 128+signum (130 for SIGINT)
// rather than the -1 that os/exec reports for signaled processes.
func TestRunSignaledExitCode(t *testing.T) {
	t.Parallel()

	// The child signals only its own PID ($$), so the test process is unaffected.
	err := Run([]string{"sh", "-c", "kill -INT $$"}, Params{
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
