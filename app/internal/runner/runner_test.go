package runner

import (
	"bytes"
	"errors"
	"os"
	"syscall"
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

// TestRunCommandNotFound verifies a command missing from PATH surfaces as an
// *exitcode.Error carrying the shell convention 127 (rather than the generic
// runtime code) and writes a diagnostic to stderr.
func TestRunCommandNotFound(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	err := Run([]string{"envx-nonexistent-command-xyz"}, Params{
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
	})
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

// -------------------------------------------------------------------------------------

// TestShouldForward verifies the interactive guard: terminal-delivered signals
// (SIGINT, SIGQUIT) are not re-forwarded when attached to a tty (the tty already
// delivered them to the child), while supervisor signals and every signal in
// non-interactive mode are forwarded.
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldForward(tt.sig, tt.interactive); got != tt.want {
				t.Errorf(
					"shouldForward(%v, interactive=%v) = %v, want %v",
					tt.sig, tt.interactive, got, tt.want,
				)
			}
		})
	}
}
