package run

import (
	"context"
	"io"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/runner"
)

// -------------------------------------------------------------------------------------

// actionParams are the positional inputs to the run action.
type actionParams struct {
	// Project is the project name to resolve.
	Project string
	// ExecArgs is the child command and its arguments to run.
	ExecArgs []string
}

// -------------------------------------------------------------------------------------

// actionConfig is the run action's composed config.
type actionConfig struct {
	config.Input
	// Overload lets file values win over existing OS environment variables.
	Overload bool
}

// -------------------------------------------------------------------------------------

// streams bundles the output sinks the run action wires the child process to.
type streams struct {
	// Stdout is the sink for the child process's standard output.
	Stdout io.Writer
	// Stderr is the sink for the child process's standard error.
	Stderr io.Writer
}

// -------------------------------------------------------------------------------------

// execute is the imperative shell: resolve the input into an envmerge.Params, build
// the merged environment, resolve the overload setting (flag > ENVX_OVERLOAD),
// then run the child process with the merged environment.
func execute(ctx context.Context, p actionParams, c *actionConfig, s streams) error {
	// resolve the input config
	ec, err := config.Resolve(&c.Input, p.Project)
	if err != nil {
		return err
	}

	// build the merged environment
	env, err := envmerge.Build(*ec)
	if err != nil {
		return err
	}

	// resolve the overload setting (flag > ENVX_OVERLOAD)
	overload := config.ResolveOverload(c.Overload, c.Changed)

	// run the child process with the merged environment; a non-zero child exit
	// code surfaces as an *exitcode.Error so main.go can propagate it.
	return runner.Run(ctx, p.ExecArgs, runner.Params{
		Env:      env.All(),
		Overload: overload,
		Stdout:   s.Stdout,
		Stderr:   s.Stderr,
	})
}
