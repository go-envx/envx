// Command envx is the entry point for the envx CLI. It executes the root
// command and maps errors to exit codes.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-envx/envx/app/internal/cli"
	"github.com/go-envx/envx/app/internal/exitcode"
)

// version is injected at build time via -ldflags.
var version = "0.0.0-dev"

// -------------------------------------------------------------------------------------
// main delegates to run and exits with the resulting code.
func main() {
	os.Exit(run())
}

// -------------------------------------------------------------------------------------
// run executes the root command and translates any error into a process exit
// code: an exitcode.Error carries its own code, every other error maps to 1.
// The run command's child-process signal handling lives in the runner, which
// forwards signals to the child and mirrors its exit status, so no root-level
// signal trapping is required here.
func run() int {
	if err := cli.NewRootCmd(version).Execute(); err != nil {
		if exitErr, ok := errors.AsType[*exitcode.Error](err); ok {
			return exitErr.Code
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}
	return 0
}
