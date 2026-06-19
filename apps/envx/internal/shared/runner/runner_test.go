package runner

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/go-envx/envx/apps/envx/internal/exitcode"
)

// -------------------------------------------------------------------------------------
// TestRunInjectsEnv verifies the merged env is passed to the child and that file
// values appear in its environment.
func TestRunInjectsEnv(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"printenv", "FROM_FILE"}, Options{
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
// TestRunPropagatesExitCode verifies a non-zero child exit surfaces as an
// exitcode.Error carrying the same code.
func TestRunPropagatesExitCode(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), []string{"sh", "-c", "exit 3"}, Options{
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
	err := Run(context.Background(), []string{"printenv", "SHARED_KEY"}, Options{
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
	err := Run(context.Background(), []string{"printenv", "SHARED_KEY"}, Options{
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

	if err := Run(context.Background(), nil, Options{}); err == nil {
		t.Fatal("expected error for empty args")
	}
}
