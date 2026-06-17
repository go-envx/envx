package app

import (
	"context"
	"io"

	"github.com/go-envx/envx/apps/envx/internal/runner"
)

// -------------------------------------------------------------------------------------
// RunOptions holds the parameters for Run that are specific to process execution.
type RunOptions struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
}

// -------------------------------------------------------------------------------------
// Run executes the full run pipeline: resolve env + exec child process.
func (a *App) Run(
	ctx context.Context,
	in PipelineInput,
	opts RunOptions,
) error {
	_, cfg, result, err := a.ResolvePipeline(in)
	if err != nil {
		return err
	}

	return a.Runner(ctx, opts.Args, runner.Options{
		Env:      result.Env,
		Overload: cfg.Overload,
		Stdout:   opts.Stdout,
		Stderr:   opts.Stderr,
	})
}
