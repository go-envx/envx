package emit

import (
	"testing"

	"github.com/go-envx/envx/app/internal/core/env"
)

// fixture bundles a Service with its fakes.
type fixture struct {
	// service is the Service under test.
	service *Service
	// explainer is the fake environment explainer.
	explainer *fakeExplainer
	// renderer is the fake renderer.
	renderer *fakeRenderer
}

// newFixture returns a Service whose explainer serves entries for project app.
func newFixture(t *testing.T, entries ...env.ExplainEntry) *fixture {
	t.Helper()
	f := &fixture{
		explainer: &fakeExplainer{
			result: env.ExplainResult{Project: "app", Entries: entries},
		},
		renderer: &fakeRenderer{},
	}
	service, err := New(Dependencies{Env: f.explainer, Renderer: f.renderer})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	f.service = service
	return f
}

// TestNewRequiresDependencies verifies New rejects a missing adapter.
func TestNewRequiresDependencies(t *testing.T) {
	t.Parallel()

	for name, deps := range map[string]Dependencies{
		"explainer": {Renderer: &fakeRenderer{}},
		"renderer":  {Env: &fakeExplainer{}},
	} {
		if _, err := New(deps); err == nil {
			t.Errorf("New without %s succeeded", name)
		}
	}
}
