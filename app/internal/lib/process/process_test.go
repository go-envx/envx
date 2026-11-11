package process

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-envx/envx/app/internal/lib/exitcode"
	"github.com/go-envx/envx/app/internal/lib/stdio"
)

// helperArg makes the test binary act as a portable child command.
const helperArg = "-envx-process-helper"

// TestMain dispatches helper invocations before running the suite.
func TestMain(m *testing.M) {
	if len(os.Args) > 2 && os.Args[1] == helperArg {
		os.Exit(runHelper(os.Args[2], os.Args[3:]))
	}
	os.Exit(m.Run())
}

// runHelper implements the child commands: exit CODE, copy (stdin to stdout,
// then "err" to stderr), getenv NAME... (space-joined values), fail N
// (N bytes of "x" to stderr, exit 3), and sleep.
func runHelper(name string, args []string) int {
	switch name {
	case "exit":
		code, _ := strconv.Atoi(args[0])
		return code
	case "fail":
		n, _ := strconv.Atoi(args[0])
		fmt.Fprint(os.Stderr, strings.Repeat("x", n))
		return 3
	case "sleep":
		time.Sleep(time.Minute)
	case "copy":
		_, _ = io.Copy(os.Stdout, os.Stdin)
		fmt.Fprintln(os.Stderr, "err")
	case "getenv":
		values := make([]string, len(args))
		for i, key := range args {
			values[i] = os.Getenv(key)
		}
		fmt.Println(strings.Join(values, " "))
	default:
		return 2
	}
	return 0
}

// helper returns the arguments that run a helper command as the child.
func helper(t *testing.T, args ...string) []string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{self, helperArg}, args...)
}

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
		Args: helper(t, "exit", "0"),
	})
	if err != nil {
		t.Fatalf("Run with nil streams: %v", err)
	}
}

func TestRunUsesInjectedStreams(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := Run(Command{
		Args: helper(t, "copy"),
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
	// A coverage-instrumented helper may append a GOCOVERDIR warning on exit.
	if got := stderr.String(); !strings.HasPrefix(got, "err\n") {
		t.Errorf("stderr = %q, want it to start with %q", got, "err\n")
	}
}

func TestRunInjectsEnv(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := Run(Command{
		Args: helper(t, "getenv", "FOO", "BAR"),
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
				Args: helper(t, "getenv", ambientKey),
				Env:  tt.env,
				Streams: stdio.Streams{
					Out: &stdout,
				},
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if got := stdout.String(); got != "\n" {
				t.Errorf("child inherited ambient var: got %q", got)
			}
		})
	}
}

func TestRunPropagatesExitCode(t *testing.T) {
	t.Parallel()

	err := Run(Command{
		Args: helper(t, "exit", "42"),
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

	// Unix rejects the missing execute bit; Windows rejects the non-PE contents.
	filePath := filepath.Join(t.TempDir(), "not_executable.exe")
	if err := os.WriteFile(filePath, []byte("not a program\n"), 0o600); err != nil {
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
