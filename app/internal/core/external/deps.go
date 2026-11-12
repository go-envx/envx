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
	// Fetch returns the plaintext for one qualified reference.
	Fetch(ctx context.Context, ref Reference) (string, error)
}

// Qualifier is an optional Provider extension that checks a reference against
// the provider's syntax and configuration and qualifies it for an environment
// without fetching.
type Qualifier interface {
	// Qualify returns ref in the fully qualified form Fetch receives, or an
	// error wrapping ErrInvalidReference.
	Qualify(ref Reference, environment string) (Reference, error)
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
