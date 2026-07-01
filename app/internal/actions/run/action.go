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

// streams bundles the output sinks the run action wires the child process to.
type streams struct {
	// Stdout is the sink for the child process's standard output.
	Stdout io.Writer
	// Stderr is the sink for the child process's standard error.
	Stderr io.Writer
}

// -------------------------------------------------------------------------------------

// execute is the imperative shell: resolve the input into an envmerge.Params, build
// the merged environment, then run the child process with the merged environment
// using the resolved overload setting.
func execute(ctx context.Context, p actionParams, in *config.Input, s streams) error {
	// resolve the input config
	resolved, err := config.Resolve(in, p.Project)
	if err != nil {
		return err
	}

	// build the merged environment
	env, err := envmerge.Build(resolved.Envmerge)
	if err != nil {
		return err
	}

	// start from the resolved runner params, then supply the merged environment
	// and output streams the config layer can't know about.
	params := resolved.Runner
	params.Env = env.All()
	params.Stdout = s.Stdout
	params.Stderr = s.Stderr

	// run the child process with the merged environment; a non-zero child exit
	// code surfaces as an *exitcode.Error so main.go can propagate it.
	return runner.Run(ctx, p.ExecArgs, params)
}
