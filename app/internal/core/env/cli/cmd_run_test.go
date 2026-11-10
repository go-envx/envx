package cli

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestRunCommandStartsChild verifies the child receives the command after
// "--", the selected environment, and the command's streams.
func TestRunCommandStartsChild(t *testing.T) {
	t.Parallel()

	factory := newFakeFactory()
	cmd := NewRunCommand(factory)
	if _, _, err := execute(
		t, cmd, "app", "--env", "production", "--", "printenv", "HOST",
	); err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if len(factory.runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(factory.runs))
	}

	run := factory.runs[0]
	if want := []string{"printenv", "HOST"}; !slices.Equal(run.Args, want) {
		t.Errorf("Args = %q, want %q", run.Args, want)
	}
	if got := run.Env["HOST"]; got != "prod-db.internal" {
		t.Errorf("HOST = %q, want prod-db.internal", got)
	}
	if run.Streams.In != cmd.InOrStdin() ||
		run.Streams.Out != cmd.OutOrStdout() ||
		run.Streams.ErrOut != cmd.ErrOrStderr() {
		t.Error("child streams are not the command's streams")
	}
}

// TestRunCommandOverload verifies host values win by default and --overload
// lets file values win.
func TestRunCommandOverload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// name labels the case.
		name string
		// args are the flags before "--".
		args []string
		// want is the child's HOST.
		want string
	}{
		{name: "host wins by default", want: "from-host"},
		{name: "overload", args: []string{"--overload"}, want: "dev-db.local"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			factory := newFakeFactory()
			factory.host = []string{"HOST=from-host"}
			args := append(append([]string{"app"}, tt.args...), "--", "true")
			if _, _, err := execute(t, NewRunCommand(factory), args...); err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			if got := factory.runs[0].Env["HOST"]; got != tt.want {
				t.Errorf("HOST = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRunCommandIgnoreErrors verifies an unresolved value aborts before the
// child starts unless --ignore-errors, which warns and omits the key.
func TestRunCommandIgnoreErrors(t *testing.T) {
	t.Parallel()

	broken := func() *fakeFactory {
		factory := newFakeFactory()
		factory.namespaces[appInclude]["broken"] = "{{NOPE}}"
		return factory
	}

	strict := broken()
	_, _, err := execute(t, NewRunCommand(strict), "app", "--", "true")
	if err == nil {
		t.Error("an unresolved value did not abort the run")
	}
	if len(strict.runs) != 0 {
		t.Error("the child started despite the unresolved value")
	}

	tolerant := broken()
	_, stderr, err := execute(
		t, NewRunCommand(tolerant), "app", "--ignore-errors", "--", "true",
	)
	if err != nil {
		t.Fatalf("Execute(--ignore-errors): %v", err)
	}
	if len(tolerant.runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(tolerant.runs))
	}
	if _, ok := tolerant.runs[0].Env["BROKEN"]; ok {
		t.Error("the unresolved key reached the child")
	}
	if !strings.Contains(stderr, "WARNING") || !strings.Contains(stderr, "BROKEN") {
		t.Errorf("stderr = %q, want a warning naming BROKEN", stderr)
	}
}

// TestRunCommandReturnsChildError verifies the child's error is returned
// unchanged so the exit status can propagate.
func TestRunCommandReturnsChildError(t *testing.T) {
	t.Parallel()

	want := errors.New("exit status 3")
	factory := newFakeFactory()
	factory.runErr = want
	_, _, err := execute(t, NewRunCommand(factory), "app", "--", "false")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// TestRunCommandArgsValidation verifies run's positional layout is enforced
// before the factory is called.
func TestRunCommandArgsValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// name labels the case.
		name string
		// args are the command-line arguments.
		args []string
		// want is a substring of the expected error.
		want string
	}{
		{"missing separator", []string{"app"}, "missing '--' separator"},
		{"no project", []string{"--", "ls"}, "exactly one project"},
		{"two projects", []string{"a", "b", "--", "ls"}, "exactly one project"},
		{"no command", []string{"app", "--"}, "no command specified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			factory := newFakeFactory()
			_, _, err := execute(t, NewRunCommand(factory), tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to contain %q", err, tt.want)
			}
			if factory.configPath != "" {
				t.Error("factory was called despite invalid arguments")
			}
		})
	}
}
