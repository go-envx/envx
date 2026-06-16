package runner

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/exitcode"
)

// -------------------------------------------------------------------------------------
// TestRunEchoCommand verifies that a simple successful command returns nil.
func TestRunEchoCommand(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell")
	}

	ctx := context.Background()
	err := Run(ctx, []string{"true"}, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// -------------------------------------------------------------------------------------
// TestRunExitCode verifies that a non-zero child exit code is surfaced as
// an exitcode.Error with the correct code.
func TestRunExitCode(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell")
	}

	ctx := context.Background()
	err := Run(ctx, []string{"sh", "-c", "exit 42"}, Options{})
	if err == nil {
		t.Fatal("expected error")
	}

	var exitErr *exitcode.Error
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected exitcode.Error, got %T: %v", err, err)
	}
	if exitErr.Code != 42 {
		t.Errorf("exit code = %d, want 42", exitErr.Code)
	}
}

// -------------------------------------------------------------------------------------
// TestRunNoCommand verifies that passing no arguments returns an error.
func TestRunNoCommand(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	err := Run(ctx, nil, Options{})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

// -------------------------------------------------------------------------------------
// TestRunWithEnv verifies that injected environment variables are visible
// to the child process.
func TestRunWithEnv(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell")
	}

	ctx := context.Background()
	env := map[string]string{"TEST_ENVX_VAR": "hello"}

	// The child should see our injected var. Use sh -c to check.
	err := Run(
		ctx,
		[]string{"sh", "-c", `[ "$TEST_ENVX_VAR" = "hello" ]`},
		Options{Env: env},
	)
	if err != nil {
		t.Fatalf("expected child to see injected env var: %v", err)
	}
}

// -------------------------------------------------------------------------------------
// TestRunOverloadMode verifies default vs overload env var precedence:
// default mode preserves OS values, overload mode lets file values win.
func TestRunOverloadMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell")
	}

	ctx := context.Background()

	// Set an OS env var, then use overload mode to override it.
	t.Setenv("TEST_ENVX_OVERLOAD", "os-value")

	// Without overload: OS wins.
	err := Run(
		ctx,
		[]string{"sh", "-c", `[ "$TEST_ENVX_OVERLOAD" = "os-value" ]`},
		Options{
			Env: map[string]string{"TEST_ENVX_OVERLOAD": "file-value"},
		},
	)
	if err != nil {
		t.Fatalf("default mode: OS env should win: %v", err)
	}

	// With overload: file wins.
	err = Run(
		ctx,
		[]string{"sh", "-c", `[ "$TEST_ENVX_OVERLOAD" = "file-value" ]`},
		Options{
			Env:      map[string]string{"TEST_ENVX_OVERLOAD": "file-value"},
			Overload: true,
		},
	)
	if err != nil {
		t.Fatalf("overload mode: file value should win: %v", err)
	}
}

// -------------------------------------------------------------------------------------
// TestBuildEnvDefault verifies that buildEnv in default mode inherits OS
// env vars and adds new file-sourced keys.
func TestBuildEnvDefault(t *testing.T) {
	t.Parallel()

	opts := Options{
		Env: map[string]string{"A": "from-file", "NEW_KEY": "new"},
	}
	result := buildEnv(opts)
	m := envToMap(result)

	// NEW_KEY should be present.
	if m["NEW_KEY"] != "new" {
		t.Errorf("NEW_KEY = %q, want %q", m["NEW_KEY"], "new")
	}

	// PATH should be inherited from OS.
	if m["PATH"] == "" {
		t.Error("expected PATH to be inherited from OS env")
	}
}

// -------------------------------------------------------------------------------------
// TestBuildEnvOverload verifies that buildEnv in overload mode lets file
// values override existing OS env vars.
func TestBuildEnvOverload(t *testing.T) {
	t.Setenv("OVERLOAD_TEST_KEY", "os-original")

	opts := Options{
		Env:      map[string]string{"OVERLOAD_TEST_KEY": "file-override"},
		Overload: true,
	}
	result := buildEnv(opts)
	m := envToMap(result)

	if m["OVERLOAD_TEST_KEY"] != "file-override" {
		t.Errorf(
			"OVERLOAD_TEST_KEY = %q, want %q",
			m["OVERLOAD_TEST_KEY"],
			"file-override",
		)
	}
}

// -------------------------------------------------------------------------------------
// TestRunContextCancellation verifies that cancelling the context kills
// the child process and returns an error.
func TestRunContextCancellation(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start a long-running child.
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, []string{"sleep", "30"}, Options{})
	}()

	// Cancel the context immediately — child should be killed.
	cancel()

	err := <-errCh
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}

	// The child should have been killed, resulting in a non-zero exit.
	var exitErr *exitcode.Error
	if !errors.As(err, &exitErr) {
		// Context cancellation may also surface as a wrapped error.
		// Either way, we got an error — success.
		return
	}
	if exitErr.Code == 0 {
		t.Error("expected non-zero exit code from cancelled child")
	}
}

// -------------------------------------------------------------------------------------
// TestRunNotFoundCommand verifies that attempting to run a nonexistent
// command returns an error.
func TestRunNotFoundCommand(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	err := Run(ctx, []string{"nonexistent-command-xyz-123"}, Options{})
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}
