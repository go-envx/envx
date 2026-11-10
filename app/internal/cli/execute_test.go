package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/exitcode"
	"github.com/go-envx/envx/app/internal/lib/stdio"
	"github.com/spf13/cobra"
)

// TestExecute verifies command outcomes map to process exit codes and that
// only failures needing a diagnostic are reported.
func TestExecute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// name describes the case.
		name string
		// args are the command-line arguments.
		args []string
		// err is the error returned by the probe command.
		err error
		// wantCode is the expected process exit code.
		wantCode int
		// wantStderr is the diagnostic expected within stderr.
		wantStderr string
	}{
		{
			name:     "success",
			args:     []string{"probe"},
			wantCode: exitOK,
		},
		{
			name:     "child exit status is propagated silently",
			args:     []string{"probe"},
			err:      exitcode.FromCode(42),
			wantCode: 42,
		},
		{
			name: "start failure reports its cause",
			args: []string{"probe"},
			err: fmt.Errorf("running: %w", exitcode.New(127,
				errors.New(`exec: "nope": executable file not found`))),
			wantCode:   127,
			wantStderr: "envx: exec: \"nope\": executable file not found\n",
		},
		{
			name:       "unknown flag is a usage failure",
			args:       []string{"probe", "--nope"},
			wantCode:   exitUsage,
			wantStderr: "ERROR: unknown flag: --nope\n",
		},
		{
			name:       "command failure is a runtime failure",
			args:       []string{"probe"},
			err:        errors.New("missing public key"),
			wantCode:   exitRuntime,
			wantStderr: "ERROR: missing public key\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			root := NewRootCommand(BuildInfo{}, &fakeFactory{}, stdio.Streams{
				Out:    &stdout,
				ErrOut: &stderr,
			})
			root.AddCommand(&cobra.Command{
				Use: "probe",
				RunE: func(*cobra.Command, []string) error {
					return tt.err
				},
			})
			root.SetArgs(tt.args)

			if got := Execute(root); got != tt.wantCode {
				t.Errorf("Execute() = %d, want %d", got, tt.wantCode)
			}
			if got := stderr.String(); !strings.Contains(got, tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", got, tt.wantStderr)
			}
			if tt.wantStderr == "" && stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}
