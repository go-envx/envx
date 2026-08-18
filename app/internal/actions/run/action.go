package run

import (
	"io"

	"github.com/go-envx/envx/app/internal/config"
	"github.com/go-envx/envx/app/internal/envmerge"
	"github.com/go-envx/envx/app/internal/runner"
)

// actionParams are the positional inputs to the run action.
type actionParams struct {
	// Project is the project name to resolve.
	Project string
	// ExecArgs is the child command and its arguments to run.
	ExecArgs []string
}

// streams bundles the output sinks the run action wires the child process to.
type streams struct {
	// Stdout is the sink for the child process's standard output.
	Stdout io.Writer
	// Stderr is the sink for the child process's standard error.
	Stderr io.Writer
}

// execute is the imperative shell: resolve the input into an envmerge.Manager,
// materialize the complete environment, then run the child process with it using
// the resolved overload setting.
func execute(p actionParams, in *config.Input, s streams) error {
	// resolve the input config, always revealing secrets because a child process
	// needs plaintext; a decryption failure fails here before the process starts.
	resolved, err := config.ResolveProject(in, p.Project, true)
	if err != nil {
		return err
	}

	// construct the manager from the resolved params
	manager, err := envmerge.New(resolved.Envmerge)
	if err != nil {
		return err
	}

	// materialize the complete environment; Materialize reveals and resolves every
	// winner and fails closed, so a child process can never receive an unresolved
	// reference as plaintext. The environment comes from the precedence-resolved
	// default the manager already carries.
	env, err := manager.Materialize("")
	if err != nil {
		return err
	}

	// start from the resolved runner params, then supply the merged environment
	// and output streams the config layer can't know about.
	params := resolved.Runner
	params.Env = env.All()
	params.Stdout = s.Stdout
	params.Stderr = s.Stderr

	// run the child process with the merged environment; the runner forwards
	// signals to the child and surfaces a non-zero or signal-terminated exit as
	// an *exitcode.Error so main.go can propagate it.
	return runner.Run(p.ExecArgs, params)
}
