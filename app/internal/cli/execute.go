package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/go-envx/envx/app/internal/lib/exitcode"
	"github.com/go-envx/envx/app/internal/lib/printer"
	"github.com/spf13/cobra"
)

// Process exit codes envx returns to the shell. Scripts branch on them, so the
// values are stable public API.
const (
	// exitOK signals successful execution.
	exitOK = 0
	// exitRuntime signals a general runtime failure (config, I/O, or resolution).
	exitRuntime = 1
	// exitUsage signals an invocation Cobra rejected before the command ran.
	exitUsage = 2
)

// Execute runs root and maps the outcome to a process exit code: a child
// process's own code is propagated verbatim via *exitcode.Error (reporting its
// cause, such as a start failure), a usage or validation error rejected before
// the command ran maps to 2, and any other failure maps to 1. Diagnostics go to
// root's error stream.
func Execute(root *cobra.Command) int {
	cmd, err := root.ExecuteC()
	if err == nil {
		return exitOK
	}

	stderr := root.ErrOrStderr()
	if exitErr, ok := errors.AsType[*exitcode.Error](err); ok {
		if exitErr.Cause != nil {
			_, _ = fmt.Fprintf(stderr, "envx: %v\n", exitErr.Cause)
		}
		return exitErr.Code
	}

	_ = reportError(stderr, err)

	// The root's PersistentPreRunE sets SilenceUsage only once parsing and
	// validation succeed.
	if !cmd.SilenceUsage {
		return exitUsage
	}
	return exitRuntime
}

// reportError writes a returned command error through the shared printer.
func reportError(w io.Writer, err error) error {
	return printer.New(printer.Options{Out: io.Discard, Err: w}).LogError(err.Error())
}
