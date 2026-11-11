package process

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestOutputNoCommand(t *testing.T) {
	t.Parallel()

	_, err := Output(t.Context(), OutputCommand{})
	if !errors.Is(err, ErrNoCommand) {
		t.Fatalf("expected ErrNoCommand, got %v", err)
	}
}

func TestOutputCapturesStdoutWithExplicitEnv(t *testing.T) {
	t.Setenv("TEST_ENVX_AMBIENT_VAR", "ambient")

	got, err := Output(t.Context(), OutputCommand{
		Args: helper(t, "getenv", "FOO", "TEST_ENVX_AMBIENT_VAR"),
		Env: map[string]string{
			"FOO": "hello",
		},
	})
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if string(got) != "hello \n" {
		t.Errorf("stdout = %q, want %q", got, "hello \n")
	}
}

func TestOutputExitErrorBoundsStderr(t *testing.T) {
	t.Parallel()

	_, err := Output(t.Context(), OutputCommand{
		Args: helper(t, "fail", "10000"),
	})
	exitErr, ok := errors.AsType[*ExitError](err)
	if !ok {
		t.Fatalf("expected *ExitError, got %T (%v)", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("Code = %d, want 3", exitErr.Code)
	}
	if len(exitErr.Stderr) != maxStderr {
		t.Errorf("len(Stderr) = %d, want %d", len(exitErr.Stderr), maxStderr)
	}
	if got := exitErr.Error(); got != "exit status 3" {
		t.Errorf("Error() = %q, want %q", got, "exit status 3")
	}
}

func TestOutputCommandNotFound(t *testing.T) {
	t.Parallel()

	_, err := Output(t.Context(), OutputCommand{
		Args: []string{"envx-nonexistent-command-xyz-12345"},
	})
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("expected exec.ErrNotFound, got %v", err)
	}
}

func TestOutputContextDeadline(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Output(ctx, OutputCommand{
		Args: helper(t, "sleep"),
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("Output returned after %v, want prompt kill", elapsed)
	}
}
