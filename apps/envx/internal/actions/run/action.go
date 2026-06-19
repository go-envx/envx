package run

import (
	"context"
	"io"

	"github.com/go-envx/envx/apps/envx/internal/engine"
	"github.com/go-envx/envx/apps/envx/internal/runner"
)

// -------------------------------------------------------------------------------------
// actionParams are the inputs to the run action: the project, the child command
// and its arguments, and the output streams to wire to the child.
type actionParams struct {
	Project  string
	ExecArgs []string
	Stdout   io.Writer
	Stderr   io.Writer
}

// -------------------------------------------------------------------------------------
// runAction is effectful by nature (it executes a child process) and so stays
// thin: it injects the resolved environment and delegates to the runner.
func runAction(
	ctx context.Context, env *engine.Result, p actionParams, overload bool,
) error {
	return runner.Run(ctx, p.ExecArgs, runner.Options{
		Env:      env.All(),
		Overload: overload,
		Stdout:   p.Stdout,
		Stderr:   p.Stderr,
	})
}
