package runner

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"syscall"
	"testing"
)

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

// TestMapToEnv verifies that a key-value map is converted into KEY=VALUE slice entries.
func TestMapToEnv(t *testing.T) {
	t.Parallel()

	envMap := map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	}
	got := mapToEnv(envMap)

	if len(got) != 2 {
		t.Fatalf("mapToEnv len = %d, want 2", len(got))
	}
	if !slices.Contains(got, "FOO=bar") {
		t.Errorf("mapToEnv missing FOO=bar, got %v", got)
	}
	if !slices.Contains(got, "BAZ=qux") {
		t.Errorf("mapToEnv missing BAZ=qux, got %v", got)
	}
}

// TestStartFailureCode verifies exit code mapping for command execution
// startup failures.
func TestStartFailureCode(t *testing.T) {
	t.Parallel()

	if code := startFailureCode(exec.ErrNotFound); code != 127 {
		t.Errorf("startFailureCode(ErrNotFound) = %d, want 127", code)
	}
	if code := startFailureCode(errors.New("permission denied")); code != 126 {
		t.Errorf("startFailureCode(permission denied) = %d, want 126", code)
	}
}
