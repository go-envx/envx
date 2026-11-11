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

// PathChecker is an optional Provider extension that checks a reference path
// against the provider's syntax and configuration without fetching.
type PathChecker interface {
	// CheckPath returns an error wrapping ErrInvalidReference for a bad path.
	CheckPath(path string) error
}

// AvailabilityChecker is an optional Provider extension that reports, without
// contacting the provider, whether it can be called.
type AvailabilityChecker interface {
	// CheckAvailable returns an error wrapping ErrProviderUnavailable when the
	// provider's configuration, credentials, or CLI are missing.
	CheckAvailable() error
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
