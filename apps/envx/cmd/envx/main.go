package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/go-envx/envx/apps/envx/internal/cmd"
)

// version is set at build time via ldflags.
var version = "0.0.0-dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	root := cmd.NewRootCmd(version)
	if err := root.ExecuteContext(ctx); err != nil {
		var exitErr *cmd.ExitCodeError
		if errors.As(err, &exitErr) {
			return exitErr.Code
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
