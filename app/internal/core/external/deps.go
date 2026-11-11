package external

import (
	"context"
	"net/http"

	"github.com/go-envx/envx/app/internal/core/value"
	"github.com/go-envx/envx/app/internal/lib/process"
)

// Provider fetches plaintext for references of one scheme.
type Provider interface {
	// Scheme returns the reference prefix without "://", such as "op".
	Scheme() string
	// Fetch returns the plaintext for one reference path.
	Fetch(ctx context.Context, path string) (string, error)
}

// ValueResolver dereferences and diagnoses values that are not external
// references.
type ValueResolver interface {
	// Resolve materializes raw under the resolver's reveal policy.
	Resolve(raw, environment string) (string, error)
	// Evaluate classifies and diagnoses raw without returning an error.
	Evaluate(raw, environment string) value.Evaluation
}

// CommandRunner runs a provider CLI and returns its stdout.
type CommandRunner func(
	ctx context.Context, command process.OutputCommand,
) ([]byte, error)

// HTTPClient sends a provider HTTP request.
type HTTPClient interface {
	// Do sends the request and returns its response.
	Do(req *http.Request) (*http.Response, error)
}
