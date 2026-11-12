package external_test

import (
	"context"
	"errors"
	"time"

	"github.com/go-envx/envx/app/internal/core/external"
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
	// qualifyErr is returned by Qualify when set.
	qualifyErr error
	// scoped makes Qualify prefix the path with the environment.
	scoped bool
	// availableErr is returned by CheckAvailable when set.
	availableErr error
	// block waits for the context to end before returning.
	block bool
	// calls records every fetched qualified path in order.
	calls []string
}

// passthrough is a next resolver that returns every value unchanged.
type passthrough struct{}

func (p *fakeProvider) Scheme() string {
	return p.scheme
}

func (p *fakeProvider) Fetch(
	ctx context.Context, ref external.Reference,
) (string, error) {
	p.calls = append(p.calls, ref.Path)
	if p.block {
		<-ctx.Done()
		return "", ctx.Err()
	}
	if p.err != nil {
		return "", p.err
	}
	plaintext, ok := p.values[ref.Path]
	if !ok {
		return "", errors.New("not found")
	}
	return plaintext, nil
}

func (p *fakeProvider) Qualify(
	ref external.Reference, environment string,
) (external.Reference, error) {
	if p.qualifyErr != nil {
		return external.Reference{}, p.qualifyErr
	}
	if p.scoped {
		ref.Path = environment + "/" + ref.Path
	}
	return ref, nil
}

func (p *fakeProvider) CheckAvailable() error {
	return p.availableErr
}

func (passthrough) Resolve(raw, _ string) (string, error) {
	return raw, nil
}

func (passthrough) Evaluate(raw, _ string) value.Evaluation {
	return value.Evaluation{Kind: value.Config, Value: raw, Resolved: true}
}

// shortTimeout keeps blocking-provider tests fast.
const shortTimeout = 10 * time.Millisecond
