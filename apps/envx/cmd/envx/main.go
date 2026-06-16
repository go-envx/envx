package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/go-envx/envx/apps/envx/internal/cmd"
	"github.com/go-envx/envx/apps/envx/internal/exitcode"
)

// -------------------------------------------------------------------------------------
// version is set at build time via ldflags.
var version = "0.0.0-dev"

// -------------------------------------------------------------------------------------
// main is the entry point for the envx CLI.
// It delegates to run, which returns an exit code.
func main() {
	os.Exit(run())
}

// -------------------------------------------------------------------------------------
// run executes the root command and returns the appropriate exit code.
func run() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	root := cmd.NewRootCmd(version)
	if err := root.ExecuteContext(ctx); err != nil {
		if exitErr, ok := errors.AsType[*exitcode.Error](err); ok {
			return exitErr.Code
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
