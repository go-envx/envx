package external_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/external"
	"github.com/go-envx/envx/app/internal/core/value"
)

// newResolver builds a resolver over passthrough with the given providers.
func newResolver(
	t *testing.T, reveal bool, providers ...*fakeProvider,
) *external.Resolver {
	t.Helper()
	registrations := make([]external.Registration, len(providers))
	for i, p := range providers {
		registrations[i] = external.Registration{Provider: p, Timeout: shortTimeout}
	}
	r, err := external.NewResolver(external.ResolverParams{
		Next:      passthrough{},
		Reveal:    reveal,
		Providers: registrations,
	})
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	return r
}

func TestNewResolverRejectsInvalidProviders(t *testing.T) {
	t.Parallel()

	tests := map[string][]external.Registration{
		"nil provider": {{}},
		"unknown":      {{Provider: &fakeProvider{scheme: "vault"}}},
		"duplicate": {
			{Provider: &fakeProvider{scheme: "op"}},
			{Provider: &fakeProvider{scheme: "op"}},
		},
	}
	for name, registrations := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := external.NewResolver(external.ResolverParams{
				Next:      passthrough{},
				Providers: registrations,
			})
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestResolveMasksWithoutFetching(t *testing.T) {
	t.Parallel()

	op := &fakeProvider{scheme: "op", values: map[string]string{"v/i/f": "pw"}}
	r := newResolver(t, false, op)

	got, err := r.Resolve("op://v/i/f", "dev")
	if err != nil || got != "op://v/i/f" {
		t.Errorf("Resolve = %q, %v; want canonical reference", got, err)
	}
	eval := r.Evaluate("op://v/i/f", "dev")
	want := value.Evaluation{Kind: value.Secret, Value: "op://v/i/f"}
	if eval != want {
		t.Errorf("Evaluate = %+v, want %+v", eval, want)
	}
	if len(op.calls) != 0 {
		t.Errorf("provider called %v while masking", op.calls)
	}
}

func TestResolveRevealsAndCaches(t *testing.T) {
	t.Parallel()

	op := &fakeProvider{scheme: "op", values: map[string]string{"v/i/f": "pw"}}
	r := newResolver(t, true, op)

	for range 2 {
		got, err := r.Resolve("op://v/i/f", "dev")
		if err != nil || got != "pw" {
			t.Fatalf("Resolve = %q, %v; want pw", got, err)
		}
	}
	eval := r.Evaluate("op://v/i/f", "dev")
	want := value.Evaluation{Kind: value.Secret, Value: "pw", Resolved: true}
	if eval != want {
		t.Errorf("Evaluate = %+v, want %+v", eval, want)
	}
	if !slices.Equal(op.calls, []string{"v/i/f"}) {
		t.Errorf("calls = %v, want one fetch", op.calls)
	}
}

func TestResolveCachesFailures(t *testing.T) {
	t.Parallel()

	op := &fakeProvider{scheme: "op", err: errors.New("denied")}
	r := newResolver(t, true, op)

	if _, err := r.Resolve("op://v/i/f", "dev"); err == nil {
		t.Fatal("expected error")
	}
	eval := r.Evaluate("op://v/i/f", "dev")
	if eval.Diagnostic.Code != diagnostic.ExternalReferenceUnresolved || eval.Resolved {
		t.Errorf("Evaluate = %+v, want unresolved finding", eval)
	}
	if len(op.calls) != 1 {
		t.Errorf("calls = %v, want one fetch", op.calls)
	}
}

func TestResolveEmptyValueIsUnresolved(t *testing.T) {
	t.Parallel()

	r := newResolver(t, true, &fakeProvider{
		scheme: "op", values: map[string]string{"v/i/f": ""},
	})
	if _, err := r.Resolve("op://v/i/f", "dev"); !errors.Is(
		err, external.ErrReferenceUnresolved,
	) {
		t.Errorf("err = %v, want ErrReferenceUnresolved", err)
	}
}

func TestResolveFailsFastAfterTimeout(t *testing.T) {
	t.Parallel()

	op := &fakeProvider{scheme: "op", block: true}
	bws := &fakeProvider{scheme: "bws", values: map[string]string{"id": "token"}}
	r := newResolver(t, true, op, bws)

	if _, err := r.Resolve("op://v/a/f", "dev"); !errors.Is(
		err, external.ErrProviderUnavailable,
	) {
		t.Fatalf("err = %v, want ErrProviderUnavailable", err)
	}
	eval := r.Evaluate("op://v/b/f", "dev")
	if eval.Diagnostic.Code != diagnostic.ExternalProviderUnavailable {
		t.Errorf("Evaluate = %+v, want unavailable finding", eval)
	}
	if len(op.calls) != 1 {
		t.Errorf("op calls = %v, want only the first", op.calls)
	}
	if got, err := r.Resolve("bws://id", "dev"); err != nil || got != "token" {
		t.Errorf("bws Resolve = %q, %v; want unaffected", got, err)
	}
}

func TestResolveInvalidReference(t *testing.T) {
	t.Parallel()

	r := newResolver(t, true, &fakeProvider{scheme: "op"})
	if _, err := r.Resolve("op://v//f", "dev"); !errors.Is(
		err, external.ErrInvalidReference,
	) {
		t.Errorf("err = %v, want ErrInvalidReference", err)
	}
	eval := r.Evaluate("op://v//f", "dev")
	if eval.Diagnostic.Code != diagnostic.InvalidExternalReference ||
		eval.Kind != value.Secret {
		t.Errorf("Evaluate = %+v, want invalid finding", eval)
	}
}

func TestResolveEscapedReference(t *testing.T) {
	t.Parallel()

	op := &fakeProvider{scheme: "op"}
	r := newResolver(t, true, op)

	got, err := r.Resolve(`\op://v/i/f`, "dev")
	if err != nil || got != "op://v/i/f" {
		t.Errorf("Resolve = %q, %v; want literal", got, err)
	}
	eval := r.Evaluate(`\op://v/i/f`, "dev")
	want := value.Evaluation{Kind: value.Config, Value: "op://v/i/f", Resolved: true}
	if eval != want {
		t.Errorf("Evaluate = %+v, want %+v", eval, want)
	}
	if len(op.calls) != 0 {
		t.Errorf("provider called %v for a literal", op.calls)
	}
}

func TestResolveUndeclaredProviderPassesThrough(t *testing.T) {
	t.Parallel()

	r := newResolver(t, true)

	got, err := r.Resolve("bws://id", "dev")
	if err != nil || got != "bws://id" {
		t.Errorf("Resolve = %q, %v; want unchanged", got, err)
	}
	eval := r.Evaluate("bws://id", "dev")
	if eval.Value != "bws://id" ||
		eval.Diagnostic.Code != diagnostic.ExternalProviderNotEnabled {
		t.Errorf("Evaluate = %+v, want not-enabled warning", eval)
	}
	if eval := r.Evaluate("plain", "dev"); !eval.Diagnostic.IsZero() {
		t.Errorf("plain value diagnosed: %+v", eval)
	}
}
