// Command envx is the entry point for the envx CLI. It wires signal handling,
// executes the root command, and maps errors to exit codes.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/go-envx/envx/apps/envx/internal/cli"
	"github.com/go-envx/envx/apps/envx/internal/exitcode"
)

// version is injected at build time via -ldflags.
var version = "0.0.0-dev"

// -------------------------------------------------------------------------------------
// main delegates to run and exits with the resulting code.
func main() {
	os.Exit(run())
}

// -------------------------------------------------------------------------------------
// run executes the root command under a signal-aware context and translates any
// error into a process exit code: an exitcode.Error carries its own code, every
// other error maps to 1.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cli.NewRootCmd(version).ExecuteContext(ctx); err != nil {
		if exitErr, ok := errors.AsType[*exitcode.Error](err); ok {
			return exitErr.Code
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}
	return 0
}
