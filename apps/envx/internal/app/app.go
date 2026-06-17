// Package app provides the core application pipeline for envx. It orchestrates
// the shared stages that most commands need: manifest discovery → project
// lookup → config resolution → namespace building → environment merging.
//
// Commands in internal/cmd/ delegate to this package after parsing CLI input.
// This separation keeps Cobra handlers thin (parse + format) while centralizing
// business logic in a Cobra-free layer that is easy to test and reuse across
// commands (run, get, set, emit, validate, etc.).
package app

import (
	"context"

	"github.com/go-envx/envx/apps/envx/internal/manifest"
	"github.com/go-envx/envx/apps/envx/internal/runner"
)

// -------------------------------------------------------------------------------------
// RunFunc is the signature for executing a child process with environment
// injection. The default is runner.Run; tests can substitute a mock.
type RunFunc func(ctx context.Context, args []string, opts runner.Options) error

// -------------------------------------------------------------------------------------
// App holds shared dependencies and provides pipeline methods. Construct via
// New() and inject into command constructors.
type App struct {
	ConfigResolver *manifest.Resolver
	Runner         RunFunc
}

// -------------------------------------------------------------------------------------
// New creates an App with production dependencies.
func New() *App {
	return &App{
		ConfigResolver: manifest.NewResolver(),
		Runner:         runner.Run,
	}
}
