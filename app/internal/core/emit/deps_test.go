package emit

import (
	"io"

	"github.com/go-envx/envx/app/internal/core/env"
)

// fakeExplainer returns a fixed result and records every request.
type fakeExplainer struct {
	// result is returned from every call.
	result env.ExplainResult
	// err, when set, fails every call.
	err error
	// calls records every request.
	calls []env.ExplainParams
}

// fakeRenderer records render calls and writes a fixed body.
type fakeRenderer struct {
	// validateErr is returned from Validate.
	validateErr error
	// rendered reports whether Render was called.
	rendered bool
	// project is the last rendered project.
	project string
	// entries are the last rendered entries.
	entries []env.ExplainEntry
}

// Explain implements Explainer.
func (f *fakeExplainer) Explain(params env.ExplainParams) (env.ExplainResult, error) {
	f.calls = append(f.calls, params)
	return f.result, f.err
}

// Validate implements Renderer.
func (r *fakeRenderer) Validate(EmitOptions) error {
	return r.validateErr
}

// Render implements Renderer.
func (r *fakeRenderer) Render(
	out io.Writer, project string, entries []env.ExplainEntry, _ EmitOptions,
) error {
	r.rendered, r.project, r.entries = true, project, entries
	_, err := io.WriteString(out, "rendered")
	return err
}
