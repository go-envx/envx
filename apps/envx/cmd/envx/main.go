package main

import (
	"context"
	"envx/internal/cmd"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Set up cancellation context for graceful shutdown (SIGINT/SIGTERM)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Execute the root command and handle errors centrally
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
