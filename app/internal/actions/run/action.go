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

// actionConfig is the run action's configurable surface: the envmerge settings it
// resolves before merging, plus the overload knob the runner consumes.
type actionConfig struct {
	// Env is the target environment to resolve.
	Env string
	// Strict requires every overlay file in the namespace chain to exist.
	Strict bool
	// Prefix is prepended to every resolved key.
	Prefix string
	// Suffix is appended to every resolved key.
	Suffix string
	// NamespacePrefix prefixes each key with its namespace name.
	NamespacePrefix bool
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

	// run the child process with the merged environment; a non-zero child exit
	// code surfaces as an *exitcode.Error so main.go can propagate it.
	return runner.Run(ctx, p.ExecArgs, runner.Params{
		Env:      env.All(),
		Overload: resolved.Overload,
		Stdout:   s.Stdout,
		Stderr:   s.Stderr,
	})
}
