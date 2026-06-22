package run

import (
	"context"
	"io"

	"github.com/go-envx/envx/apps/envx/internal/config"
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
// actionConfig is the run action's composed config: the shared resolution input
// plus the run-local --overload toggle.
type actionConfig struct {
	config.Input
	Overload bool
}

// -------------------------------------------------------------------------------------
// execute is the imperative shell: resolve the input into an engine.Config, build
// the merged environment, resolve the overload setting (flag > ENVX_OVERLOAD),
// then exec the child process with the merged environment.
func execute(ctx context.Context, p actionParams, c *actionConfig) error {
	ec, err := config.Resolve(&c.Input, p.Project)
	if err != nil {
		return err
	}
	env, err := engine.Build(ec)
	if err != nil {
		return err
	}
	overload := config.ResolveOverload(c.Overload, c.Changed)
	return runAction(ctx, env, p, overload)
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
