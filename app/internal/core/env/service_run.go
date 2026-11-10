package env

import (
	"errors"
	"slices"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/process"
	"github.com/go-envx/envx/app/internal/lib/stdio"
)

// RunParams selects the environment and the child command.
type RunParams struct {
	// Project names the declared project; empty selects the only project.
	Project string
	// Args is the command followed by its arguments.
	Args []string
	// Streams are the child's standard streams; nil streams are EOF or discard.
	Streams stdio.Streams
	// IgnoreErrors downgrades per-key failures to warnings.
	IgnoreErrors bool
	// OnWarning presents each downgraded failure before the child starts; an
	// error aborts the run. Nil discards warnings.
	OnWarning func(diagnostic.Finding) error
	// Options are setting overrides that win over the manifest.
	Options workspace.Settings
}

// Run materializes the environment and executes the command with exactly that
// environment. The child's exit status is returned by the executor.
func (s *Service) Run(params RunParams) error {
	if s.deps.ProcessRunner == nil {
		return errors.New("executor is required")
	}
	if len(params.Args) == 0 {
		return process.ErrNoCommand
	}
	result, err := s.Materialize(MaterializeParams{
		Project:      params.Project,
		IgnoreErrors: params.IgnoreErrors,
		Options:      params.Options,
	})
	if err != nil {
		return err
	}
	if params.OnWarning != nil {
		for _, warning := range result.Warnings {
			if err := params.OnWarning(warning); err != nil {
				return err
			}
		}
	}
	return s.deps.ProcessRunner(process.Command{
		Args:    slices.Clone(params.Args),
		Env:     result.Environment.All(),
		Streams: params.Streams.Normalized(),
	})
}
