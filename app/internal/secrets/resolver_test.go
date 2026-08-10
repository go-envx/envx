package secrets

import (
	"path/filepath"
	"strings"
	"testing"
)

// newTestResolver builds a Resolver over an in-memory store for resolution tests.
func newTestResolver() *Resolver {
	return &Resolver{
		values: map[reference]string{
			{group: "production", key: "postgres_password"}: "prod-pw",
			{group: "shared", key: "api_key"}:               "shared-key",
		},
	}
}

// TestManagerResolverLoadsSecrets verifies Manager.Resolver reads a valid
// document and makes its entries available to reference resolution.
func TestManagerResolverLoadsSecrets(t *testing.T) {
	t.Parallel()

	storePath := writeStore(t,
		"secrets:\n  production:\n    postgres_password: prod-pw\n",
	)
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
		Cipher:                newTestCipher(t),
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	r, err := manager.Resolver()
	if err != nil {
		t.Fatalf("Resolver(): %v", err)
	}

	got, err := r.Resolve("secret://production/postgres_password", "")
	if err != nil {
		t.Fatalf("Resolve(): %v", err)
	}
	if got != "prod-pw" {
		t.Errorf("Resolve() = %q, want %q", got, "prod-pw")
	}
}

// TestManagerResolverMissingFileIsEmpty verifies an absent secrets file is
// optional and produces a resolver that reports references as dangling.
func TestManagerResolverMissingFileIsEmpty(t *testing.T) {
	t.Parallel()

	manager, err := New(Params{
		SecretsPath:           filepath.Join(t.TempDir(), "nope.yaml"),
		KeysPath:              filepath.Join(t.TempDir(), "envx.keys"),
		Cipher:                newTestCipher(t),
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New() absent: %v", err)
	}
	r, err := manager.Resolver()
	if err != nil {
		t.Fatalf("Resolver() absent: %v", err)
	}
	if _, err := r.Resolve("secret://any/thing", ""); err == nil {
		t.Error("expected a missing-file resolver to reject a dangling reference")
	}
}

// TestManagerResolverMalformed verifies a malformed secrets file is an error.
func TestManagerResolverMalformed(t *testing.T) {
	t.Parallel()

	storePath := writeStore(t, "{")
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
		Cipher:                newTestCipher(t),
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if _, err := manager.Resolver(); err == nil {
		t.Error("expected Resolver() to reject a malformed secrets file")
	}
}

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

// TestResolveGroupCaseInsensitive verifies stored and referenced group names
// share the document-wide case-insensitive identity policy.
func TestResolveGroupCaseInsensitive(t *testing.T) {
	t.Parallel()

	path := writeStore(t, "secrets:\n  Production:\n    token: value\n")
	manager, err := New(Params{
		SecretsPath:           path,
		KeysPath:              filepath.Join(filepath.Dir(path), "envx.keys"),
		Cipher:                newTestCipher(t),
		PrivateKeyResolver:    newPrivateKeyTestResolver(),
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	r, err := manager.Resolver()
	if err != nil {
		t.Fatalf("Resolver() error = %v", err)
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
