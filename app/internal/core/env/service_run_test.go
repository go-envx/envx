package env

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/lib/process"
	"github.com/go-envx/envx/app/internal/lib/stdio"
)

// runFixture serves a namespace with one secret and records executions.
func runFixture(t *testing.T, yaml string) (*Service, *fakeExecutor) {
	t.Helper()
	executor := &fakeExecutor{}
	opener := &fakeOpener{plaintext: map[string]string{"secret://db/password": "s3cr3t"}}
	return newService(t, testWorkspace(), Dependencies{
		EnvNamespaces:    newFakeNamespaces().base(t, appInclude, yaml),
		HostEnv:          hostOf("HOST_ONLY=x"),
		NewValueResolver: opener.open,
		ProcessRunner:    executor.run,
	}), executor
}

// TestRun verifies the child receives the materialized environment, a copy of
// the arguments, and normalized streams.
func TestRun(t *testing.T) {
	t.Parallel()

	s, executor := runFixture(t, "password: secret://db/password")
	var out bytes.Buffer
	args := []string{"echo", "hi"}
	err := s.Run(RunParams{Args: args, Streams: stdio.Streams{Out: &out}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	cmd := executor.command
	if !slices.Equal(cmd.Args, args) || cmd.Env["PASSWORD"] != "s3cr3t" ||
		cmd.Env["HOST_ONLY"] != "x" {
		t.Errorf("command = %+v", cmd)
	}
	args[0] = "mutated"
	if cmd.Args[0] != "echo" {
		t.Error("command args alias the caller's slice")
	}
	if cmd.Streams.Out != &out || cmd.Streams.In == nil || cmd.Streams.ErrOut == nil {
		t.Errorf("streams = %+v, want normalized with caller output", cmd.Streams)
	}
}

// TestRunPropagatesExitStatus verifies executor errors are returned unchanged.
func TestRunPropagatesExitStatus(t *testing.T) {
	t.Parallel()

	s, executor := runFixture(t, "a: b")
	executor.err = errors.New("exit 3")
	if err := s.Run(RunParams{Args: []string{"x"}}); !errors.Is(err, executor.err) {
		t.Errorf("Run error = %v, want executor error", err)
	}
}

// TestRunWarnings verifies warnings are presented before the child starts and
// a presentation error aborts it.
func TestRunWarnings(t *testing.T) {
	t.Parallel()

	s, executor := runFixture(t, "dangling: secret://db/missing\nok: fine")
	var seen []diagnostic.Finding
	err := s.Run(RunParams{
		Args:         []string{"x"},
		IgnoreErrors: true,
		OnWarning: func(f diagnostic.Finding) error {
			if executor.calls != 0 {
				t.Error("warning presented after the child started")
			}
			seen = append(seen, f)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(seen) != 1 || seen[0].Key != "DANGLING" {
		t.Errorf("warnings = %+v", seen)
	}
	if _, ok := executor.command.Env["DANGLING"]; ok {
		t.Error("failed key passed to the child")
	}

	errPresent := errors.New("write failed")
	executor.calls = 0
	err = s.Run(RunParams{
		Args:         []string{"x"},
		IgnoreErrors: true,
		OnWarning:    func(diagnostic.Finding) error { return errPresent },
	})
	if !errors.Is(err, errPresent) || executor.calls != 0 {
		t.Errorf("Run error = %v, calls = %d", err, executor.calls)
	}
}

// TestRunFailures verifies missing dependencies, arguments, and strict
// resolution failures never start a child.
func TestRunFailures(t *testing.T) {
	t.Parallel()

	s, executor := runFixture(t, "dangling: secret://db/missing")
	if err := s.Run(RunParams{}); !errors.Is(err, process.ErrNoCommand) {
		t.Errorf("Run without args = %v, want ErrNoCommand", err)
	}
	if err := s.Run(RunParams{Args: []string{"x"}}); err == nil {
		t.Error("Run with unresolved value succeeded")
	}
	if executor.calls != 0 {
		t.Errorf("executor calls = %d, want 0", executor.calls)
	}

	noExecutor := newService(
		t, testWorkspace(), Dependencies{EnvNamespaces: newFakeNamespaces()},
	)
	if err := noExecutor.Run(RunParams{Args: []string{"x"}}); err == nil {
		t.Error("Run without Executor succeeded")
	}
}
