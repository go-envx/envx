package external

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/value"
)

// DefaultTimeout bounds each provider call when a registration sets none.
const DefaultTimeout = 30 * time.Second

// ResolverParams configures one operation-scoped resolver.
type ResolverParams struct {
	// Next resolves every value that is not a reference to an enabled provider.
	Next ValueResolver
	// Reveal fetches values instead of masking them.
	Reveal bool
	// Providers are the enabled providers, at most one per scheme.
	Providers []Registration
}

// Registration enables one provider.
type Registration struct {
	// Provider fetches the scheme's references.
	Provider Provider
	// Timeout bounds each fetch; zero means DefaultTimeout.
	Timeout time.Duration
}

// Resolver dereferences external references for one operation and delegates
// every other value to the next resolver. A masking resolver never contacts a
// provider.
type Resolver struct {
	// next resolves values that are not references to enabled providers.
	next ValueResolver
	// reveal fetches values instead of masking them.
	reveal bool
	// providers holds the enabled providers by scheme.
	providers map[string]*providerState
	// cache holds fetch outcomes, including failures, by reference.
	cache map[Reference]fetchResult
}

// providerState tracks one enabled provider within the operation.
type providerState struct {
	// provider fetches the scheme's references.
	provider Provider
	// timeout bounds each fetch.
	timeout time.Duration
	// timedOut makes later fetches fail fast after the first timeout.
	timedOut bool
}

// fetchResult caches one fetch outcome.
type fetchResult struct {
	// plaintext is the fetched value.
	plaintext string
	// err is the fetch failure, if any.
	err error
}

// NewResolver constructs an operation-scoped resolver without fetching.
func NewResolver(params ResolverParams) (*Resolver, error) {
	if params.Next == nil {
		return nil, errors.New("next resolver is required")
	}
	providers := make(map[string]*providerState, len(params.Providers))
	for _, reg := range params.Providers {
		if reg.Provider == nil {
			return nil, errors.New("provider is required")
		}
		scheme := reg.Provider.Scheme()
		if !slices.Contains(schemes, scheme) {
			return nil, fmt.Errorf("unsupported external provider %q", scheme)
		}
		if _, dup := providers[scheme]; dup {
			return nil, fmt.Errorf("external provider %q registered twice", scheme)
		}
		timeout := reg.Timeout
		if timeout <= 0 {
			timeout = DefaultTimeout
		}
		providers[scheme] = &providerState{provider: reg.Provider, timeout: timeout}
	}
	return &Resolver{
		next:      params.Next,
		reveal:    params.Reveal,
		providers: providers,
		cache:     make(map[Reference]fetchResult),
	}, nil
}

// Resolve masks an enabled provider's reference to its canonical form or, when
// revealing, fetches it. A leading backslash escapes a literal reference.
func (r *Resolver) Resolve(raw, environment string) (string, error) {
	literal, escaped, enabled := r.match(raw)
	if !enabled {
		return r.next.Resolve(raw, environment)
	}
	if escaped {
		return literal, nil
	}
	ref, err := ParseReference(literal)
	if err != nil {
		return "", err
	}
	if !r.reveal {
		return ref.String(), nil
	}
	return r.fetch(ref)
}

// Evaluate classifies and diagnoses a value without returning an error. A
// reference to an undeclared provider passes through with a warning.
func (r *Resolver) Evaluate(raw, environment string) value.Evaluation {
	literal, escaped, enabled := r.match(raw)
	if !enabled {
		eval := r.next.Evaluate(raw, environment)
		if _, ok := schemeOf(raw); ok && eval.Diagnostic.IsZero() {
			eval.Diagnostic = finding(
				diagnostic.ExternalProviderNotEnabled,
				"the provider for this reference is not declared under secrets.providers",
			)
		}
		return eval
	}
	if escaped {
		return value.Evaluation{Kind: value.Config, Value: literal, Resolved: true}
	}
	ref, err := ParseReference(literal)
	if err != nil {
		return value.Evaluation{
			Kind:       value.Secret,
			Value:      raw,
			Diagnostic: finding(diagnostic.InvalidExternalReference, err.Error()),
		}
	}
	eval := value.Evaluation{Kind: value.Secret, Value: ref.String()}
	if !r.reveal {
		return eval
	}
	plaintext, err := r.fetch(ref)
	if err != nil {
		eval.Diagnostic = fetchFinding(err)
		return eval
	}
	eval.Value = plaintext
	eval.Resolved = true
	return eval
}

// match reports whether raw, after an optional escaping backslash, uses an
// enabled provider's scheme.
func (r *Resolver) match(raw string) (literal string, escaped, enabled bool) {
	literal, escaped = strings.CutPrefix(raw, `\`)
	scheme, ok := schemeOf(literal)
	if !ok {
		return raw, false, false
	}
	_, enabled = r.providers[scheme]
	return literal, escaped, enabled
}

// fetch returns a reference's cached outcome, fetching on a miss. After a
// provider's first timeout, its remaining references fail without a call.
func (r *Resolver) fetch(ref Reference) (string, error) {
	if cached, ok := r.cache[ref]; ok {
		return cached.plaintext, cached.err
	}
	state := r.providers[ref.Scheme]
	var result fetchResult
	if state.timedOut {
		result.err = state.timeoutError()
	} else {
		result = state.fetch(ref)
	}
	r.cache[ref] = result
	return result.plaintext, result.err
}

// fetch calls the provider under its timeout, never exposing provider output
// beyond the provider's own error.
func (s *providerState) fetch(ref Reference) fetchResult {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	plaintext, err := s.provider.Fetch(ctx, ref.Path)
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		s.timedOut = true
		return fetchResult{err: s.timeoutError()}
	case err != nil:
		return fetchResult{err: fmt.Errorf("resolving %s: %w", ref, err)}
	case plaintext == "":
		return fetchResult{
			err: fmt.Errorf("resolving %s: %w: empty value", ref, ErrReferenceUnresolved),
		}
	default:
		return fetchResult{plaintext: plaintext}
	}
}

// timeoutError reports the provider's earlier or current timeout.
func (s *providerState) timeoutError() error {
	return fmt.Errorf(
		"%w: %s timed out after %s", ErrProviderUnavailable, s.provider.Scheme(), s.timeout,
	)
}

// fetchFinding maps a fetch failure onto a finding.
func fetchFinding(err error) diagnostic.Finding {
	if errors.Is(err, ErrProviderUnavailable) {
		return finding(diagnostic.ExternalProviderUnavailable, err.Error())
	}
	return finding(diagnostic.ExternalReferenceUnresolved, err.Error())
}

// finding builds a finding at the code's default severity.
func finding(code diagnostic.Code, message string) diagnostic.Finding {
	def, _ := diagnostic.Lookup(code)
	return diagnostic.Finding{Code: code, Level: def.Level, Message: message}
}
