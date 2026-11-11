package external_test

import (
	"context"
	"errors"
	"time"

	"github.com/go-envx/envx/app/internal/core/value"
)

// fakeProvider returns canned values and records fetched paths.
type fakeProvider struct {
	// scheme is the provider's scheme.
	scheme string
	// values maps paths to plaintext.
	values map[string]string
	// err is returned for every fetch when set.
	err error
	// block waits for the context to end before returning.
	block bool
	// calls records every fetched path in order.
	calls []string
}

// passthrough is a next resolver that returns every value unchanged.
type passthrough struct{}

func (p *fakeProvider) Scheme() string {
	return p.scheme
}

func (p *fakeProvider) Fetch(ctx context.Context, path string) (string, error) {
	p.calls = append(p.calls, path)
	if p.block {
		<-ctx.Done()
		return "", ctx.Err()
	}
	if p.err != nil {
		return "", p.err
	}
	plaintext, ok := p.values[path]
	if !ok {
		return "", errors.New("not found")
	}
	return plaintext, nil
}

func (passthrough) Resolve(raw, _ string) (string, error) {
	return raw, nil
}

func (passthrough) Evaluate(raw, _ string) value.Evaluation {
	return value.Evaluation{Kind: value.Config, Value: raw, Resolved: true}
}

// shortTimeout keeps blocking-provider tests fast.
const shortTimeout = 10 * time.Millisecond
