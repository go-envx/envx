package emit

import (
	"io"

	"github.com/go-envx/envx/app/internal/core/env"
)

// Explainer diagnoses one project environment without aborting per value.
type Explainer interface {
	// Explain diagnoses the selected winners of one project environment.
	Explain(params env.ExplainParams) (env.ExplainResult, error)
}

// Renderer serializes resolved entries to an emit target.
type Renderer interface {
	// Validate rejects unsupported targets and option combinations.
	Validate(options EmitOptions) error
	// Render writes entries for project in the target's format.
	Render(
		out io.Writer, project string, entries []env.ExplainEntry, options EmitOptions,
	) error
}

// Dependencies are the adapters a Service uses; both are required.
type Dependencies struct {
	// Env explains the environment with reveal.
	Env Explainer
	// Renderer serializes the explained entries.
	Renderer Renderer
}
