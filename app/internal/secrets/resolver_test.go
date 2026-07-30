package secrets

import (
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------

// newTestResolver builds a Resolver over an in-memory store for resolution tests.
func newTestResolver() *Resolver {
	return &Resolver{
		store: &store{
			secrets: map[reference]string{
				{group: "production", key: "postgres_password"}: "prod-pw",
				{group: "shared", key: "api_key"}:               "shared-key",
			},
		},
	}
}

// -------------------------------------------------------------------------------------

// TestResolve verifies plain values pass through and references dereference,
// covering explicit groups, the shared group, and the backslash escape hatch.
func TestResolve(t *testing.T) {
	t.Parallel()

	r := newTestResolver()
	tests := []struct {
		name, value, env, want string
	}{
		{"plain value", "localhost", "production", "localhost"},
		{"url passes through", "postgres://u@h/db", "production", "postgres://u@h/db"},
		{"explicit group", "secret://production/postgres_password", "dev", "prod-pw"},
		{"shared group", "secret://shared/api_key", "production", "shared-key"},
		{"escaped literal", `\secret://x`, "production", "secret://x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := r.Resolve(tt.value, tt.env)
			if err != nil {
				t.Fatalf("Resolve(%q): %v", tt.value, err)
			}
			if got != tt.want {
				t.Errorf("Resolve(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------------------

// TestResolveErrors verifies dangling references and malformed reference forms
// fail loudly rather than leaking the raw reference string.
func TestResolveErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, value, env string
	}{
		{"dangling key", "secret://production/missing", "production"},
		{"dangling group", "secret://ghost/key", "production"},
		{"group-less key", "secret://postgres_password", "production"},
		{"too many slashes", "secret://a/b/c", "production"},
		{"empty reference", "secret://", "production"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newTestResolver()
			if _, err := r.Resolve(tt.value, tt.env); err == nil {
				t.Errorf("Resolve(%q) expected error", tt.value)
			}
		})
	}
}

// -------------------------------------------------------------------------------------

// TestResolveEmptyReferenceMessage verifies an empty reference reports the empty
// fault rather than a generic malformed-reference error.
func TestResolveEmptyReferenceMessage(t *testing.T) {
	t.Parallel()

	r := newTestResolver()
	_, err := r.Resolve("secret://", "production")
	if err == nil {
		t.Fatal("expected error for empty reference")
	}
	if !strings.Contains(err.Error(), "empty secret reference") {
		t.Errorf("error = %v, want it to mention an empty secret reference", err)
	}
}
