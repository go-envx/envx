package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------

// writeStore writes body to a secrets.yaml in a fresh temp dir and returns its path.
func writeStore(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// -------------------------------------------------------------------------------------

// newTestResolver builds a Resolver over an in-memory store for resolution tests.
func newTestResolver() *Resolver {
	return &Resolver{
		values: map[reference]string{
			{group: "production", key: "postgres_password"}: "prod-pw",
			{group: "shared", key: "api_key"}:               "shared-key",
		},
	}
}

// -------------------------------------------------------------------------------------

// TestOpenLoadsSecrets verifies Open reads a valid document and makes its
// entries available to reference resolution.
func TestOpenLoadsSecrets(t *testing.T) {
	t.Parallel()

	r, err := Open(Params{SecretsPath: writeStore(t,
		"secrets:\n  production:\n    postgres_password: prod-pw\n",
	)})
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}

	got, err := r.Resolve("secret://production/postgres_password", "")
	if err != nil {
		t.Fatalf("Resolve(): %v", err)
	}
	if got != "prod-pw" {
		t.Errorf("Resolve() = %q, want %q", got, "prod-pw")
	}
}

// -------------------------------------------------------------------------------------

// TestOpenMissingFileIsEmpty verifies an absent secrets file is optional and
// produces a resolver that reports references as dangling.
func TestOpenMissingFileIsEmpty(t *testing.T) {
	t.Parallel()

	r, err := Open(Params{SecretsPath: filepath.Join(t.TempDir(), "nope.yaml")})
	if err != nil {
		t.Fatalf("Open() absent: %v", err)
	}
	if _, err := r.Resolve("secret://any/thing", ""); err == nil {
		t.Error("expected a missing-file resolver to reject a dangling reference")
	}
}

// -------------------------------------------------------------------------------------

// TestOpenMalformed verifies a malformed secrets file is an error.
func TestOpenMalformed(t *testing.T) {
	t.Parallel()

	if _, err := Open(Params{SecretsPath: writeStore(t, "{")}); err == nil {
		t.Error("expected Open() to reject a malformed secrets file")
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

// TestResolveGroupCaseInsensitive verifies stored and referenced group names
// share the document-wide case-insensitive identity policy.
func TestResolveGroupCaseInsensitive(t *testing.T) {
	t.Parallel()

	path := writeStore(t, "secrets:\n  Production:\n    token: value\n")
	r, err := Open(Params{SecretsPath: path})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	for _, value := range []string{
		"secret://production/token",
		"secret://PRODUCTION/token",
	} {
		got, err := r.Resolve(value, "")
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", value, err)
		}
		if got != "value" {
			t.Errorf("Resolve(%q) = %q, want %q", value, got, "value")
		}
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
