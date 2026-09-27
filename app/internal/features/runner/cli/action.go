package cli

import (
	"github.com/go-envx/envx/app/internal/core"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/runner"
	"github.com/go-envx/envx/app/internal/utils/printer"
)

// runnerService defines the process execution contract required by the run action.
type runnerService interface {
	Run(params runner.RunParams) error
}

// actionParams are the positional inputs to the run action.
type actionParams struct {
	// Project is the project name to resolve.
	Project string
	// ExecArgs is the child command and its arguments to run.
	ExecArgs []string
	// IgnoreErrors downgrades resolution failures to warnings and omits the failing
	// keys so the child process still starts.
	IgnoreErrors bool
}

// execute is the imperative shell: resolve the input into an env.Manager,
// materialize the complete effective environment, then run the child process with
// it. Overload and OS composition are settled inside Materialize, so the runner
// receives a ready environment.
func execute(
	r runnerService, p actionParams, in *core.Input, pr *printer.Printer,
) error {
	// resolve the input config
	resolved, err := core.ResolveProject(in, p.Project)
	if err != nil {
		return err
	}

	// materialize the complete environment; by default a single unresolved value
	// aborts here, before the child starts, so it can never receive an unresolved
	// reference as plaintext. Under --ignore-errors each resolution failure is
	// downgraded to a stderr warning and its key is omitted instead, so the child
	// still starts and inherits the omitted key from the ambient environment.
	// Structural failures stay fatal in both modes.
	result, err := resolved.Envmerge.Materialize(env.MaterializeParams{
		IgnoreErrors: p.IgnoreErrors,
	})
	if err != nil {
		return err
	}
	for _, warning := range result.Warnings {
		_ = pr.LogWarning(warning.Error())
	}

	// run the child process with the ready environment; the runner injects it
	// verbatim, forwards signals to the child, and surfaces a non-zero or
	// signal-terminated exit as an *exitcode.Error so main.go can propagate it.
	return r.Run(runner.RunParams{
		Args: p.ExecArgs,
		Env:  result.Environment.All(),
	})
}
