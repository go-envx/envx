package emit

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/value"
	"github.com/go-envx/envx/app/internal/core/workspace"
)

// resolved returns an entry whose evaluation resolved to text.
func resolved(key, text string, secret bool) env.ExplainEntry {
	return env.ExplainEntry{
		Key:        key,
		Secret:     secret,
		Evaluation: value.Evaluation{Value: text, Resolved: true},
	}
}

// TestEmit verifies emit explains with reveal and the caller's settings,
// renders for the explained project, and reports secret content per slice.
func TestEmit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// only selects the slice.
		only string
		// want reports whether the result contains secrets.
		want bool
	}{
		{only: "", want: true},
		{only: OnlySecrets, want: true},
		{only: OnlyConfig, want: false},
	}
	for _, tt := range tests {
		f := newFixture(t,
			resolved("PASSWORD", "s3cr3t", true),
			resolved("PLAIN", "p", false),
		)
		environment := "production"
		result, err := f.service.Emit(EmitParams{
			Options:  EmitOptions{Only: tt.only},
			Settings: workspace.Settings{Env: &environment},
		})
		if err != nil {
			t.Fatalf("Emit(%q): %v", tt.only, err)
		}
		if string(result.Data) != "rendered" || result.ContainsSecrets != tt.want {
			t.Errorf("Emit(%q) = %+v, want secrets %v", tt.only, result, tt.want)
		}
		if f.renderer.project != "app" || len(f.renderer.entries) != 2 {
			t.Errorf("rendered %q with %d entries", f.renderer.project, len(f.renderer.entries))
		}
		call := f.explainer.calls[0]
		if !call.Reveal || call.Key != "" || call.Options.Env != &environment {
			t.Errorf("Explain params = %+v", call)
		}
	}
}

// TestEmitFailsClosed verifies option errors precede explanation, and
// explanation failures and unresolved values prevent rendering.
func TestEmitFailsClosed(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.renderer.validateErr = errors.New("bad target")
	if _, err := f.service.Emit(EmitParams{}); !errors.Is(err, f.renderer.validateErr) {
		t.Errorf("Emit error = %v, want validation error", err)
	}
	if len(f.explainer.calls) != 0 {
		t.Errorf("explained %d times before validation passed", len(f.explainer.calls))
	}

	f = newFixture(t)
	f.explainer.err = env.ErrProjectNotFound
	if _, err := f.service.Emit(EmitParams{}); !errors.Is(err, env.ErrProjectNotFound) {
		t.Errorf("Emit error = %v, want the explain error", err)
	}

	f = newFixture(t,
		env.ExplainEntry{Key: "A"},
		env.ExplainEntry{Key: "B", Secret: true},
		resolved("OK", "fine", false),
	)
	_, err := f.service.Emit(EmitParams{})
	if !errors.Is(err, env.ErrUnresolved) ||
		!strings.Contains(err.Error(), "2 value(s)") ||
		!strings.Contains(err.Error(), "A, B") {
		t.Errorf("Emit error = %v", err)
	}
	if f.renderer.rendered {
		t.Error("renderer called despite unresolved values")
	}
}
