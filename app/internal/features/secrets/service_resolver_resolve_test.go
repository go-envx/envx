package secrets

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/features/privatekey"
	"github.com/go-envx/envx/app/internal/shared/value"
)

// newMaskingResolver builds a masking Resolver over an in-memory store. A masking
// resolver never touches key material, so its cipher and private-key resolver are
// left nil.
func newMaskingResolver() *Resolver {
	return &Resolver{
		values: map[reference]string{
			{group: "production", key: "postgres_password"}: "encrypted-age:abc",
			{group: "shared", key: "api_key"}:               "encrypted-age:def",
		},
		resolvedKeys: map[string]string{},
	}
}

// recordingResolver resolves per-group private keys and counts each lookup, so a
// test can assert lazy, cached, group-scoped resolution.
type recordingResolver struct {
	// keys maps a normalized group to its private-key material.
	keys map[string]string
	// calls counts how many times each group was resolved.
	calls map[string]int
}

// Location returns a test location.
func (r *recordingResolver) Location() string { return "test-keys-location" }

// Resolve returns a group's private key, recording the lookup. A group absent
// from keys reports that no key is available.
func (r *recordingResolver) Resolve(group string) (privatekey.PrivateKey, error) {
	r.calls[group]++
	key, ok := r.keys[group]
	if !ok {
		return privatekey.PrivateKey{}, privatekey.ErrNotAvailable
	}
	return privatekey.PrivateKey{Value: key, Origin: "test"}, nil
}

// Set accepts private-key material without storing it.
func (r *recordingResolver) Set(string, string) error { return nil }

// TestOpenResolverImplementsEvaluator verifies the opened resolver can also
// evaluate values, which env.Explain relies on.
func TestOpenResolverImplementsEvaluator(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestService())
	r, err := manager.OpenResolver(false)
	if err != nil {
		t.Fatalf("OpenResolver(): %v", err)
	}
	if _, ok := r.(value.Evaluator); !ok {
		t.Errorf("OpenResolver() = %T, want a value.Evaluator", r)
	}
}

// TestManagerResolverMasksByDefault verifies the default resolver masks
// references without loading any private key, even when none is available.
func TestManagerResolverMasksByDefault(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestService())
	_, err := manager.SetSecret(
		"production", "database_password", func() (string, error) {
			return "database-password", nil
		},
	)
	if err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	r, err := manager.OpenResolver(false)
	if err != nil {
		t.Fatalf("OpenResolver(): %v", err)
	}
	got, err := r.Resolve("secret://production/database_password", "")
	if err != nil {
		t.Fatalf("Resolve(): %v", err)
	}
	if want := "secret://production/database_password"; got != want {
		t.Errorf("Resolve() = %q, want masked reference %q", got, want)
	}
}

// TestManagerResolverRevealsStoredSecret verifies a revealing resolver decrypts a
// stored value back to its plaintext.
func TestManagerResolverRevealsStoredSecret(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, fixedPrivateKeyResolver{})
	const plaintext = "database-password"
	_, err := manager.SetSecret(
		"production", "database_password", func() (string, error) {
			return plaintext, nil
		},
	)
	if err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	r, err := manager.OpenResolver(true)
	if err != nil {
		t.Fatalf("OpenResolver(): %v", err)
	}
	got, err := r.Resolve("secret://Production/database_password", "")
	if err != nil {
		t.Fatalf("Resolve(): %v", err)
	}
	if got != plaintext {
		t.Errorf("Resolve() = %q, want %q", got, plaintext)
	}
}

// TestManagerResolverRevealUnavailableKeyFails verifies revealing a reference
// fails loudly when no private key is available, since it promises plaintext.
func TestManagerResolverRevealUnavailableKeyFails(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())
	_, err := manager.SetSecret(
		"production", "database_password", func() (string, error) {
			return "database-password", nil
		},
	)
	if err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}

	r, err := manager.OpenResolver(true)
	if err != nil {
		t.Fatalf("OpenResolver(): %v", err)
	}
	if _, err := r.Resolve("secret://production/database_password", ""); err == nil {
		t.Fatal("Resolve() revealed a secret without an available private key")
	}
}

// TestManagerResolverRevealsLazilyByGroup verifies a revealing resolver resolves
// a private key only for referenced groups and caches it per group: an
// unreferenced group's missing key never blocks resolution, and a repeated
// reference resolves its key only once.
func TestManagerResolverRevealsLazilyByGroup(t *testing.T) {
	t.Parallel()

	selected := newTestCipher(t)
	prodPub, prodPriv := testKeys(t, selected)
	sharedPub, _ := testKeys(t, selected)

	// Only the production group's private key is available; shared's is absent to
	// prove its key is never requested when no reference reveals it.
	resolver := &recordingResolver{
		keys:  map[string]string{"production": prodPriv},
		calls: map[string]int{},
	}
	storePath := writeStore(t,
		"public-keys:\n  production: "+prodPub+
			"\n  shared: "+sharedPub+"\n",
	)
	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, storePath),
		Cipher:            selected,
		PrivateKeyService: resolver,
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if _, err := manager.SetSecret("production", "token", func() (string, error) {
		return "prod-token", nil
	}); err != nil {
		t.Fatalf("SetSecret() production: %v", err)
	}
	if _, err := manager.SetSecret("shared", "token", func() (string, error) {
		return "shared-token", nil
	}); err != nil {
		t.Fatalf("SetSecret() shared: %v", err)
	}

	r, err := manager.OpenResolver(true)
	if err != nil {
		t.Fatalf("OpenResolver(): %v", err)
	}
	for range 2 {
		got, err := r.Resolve("secret://production/token", "")
		if err != nil {
			t.Fatalf("Resolve(): %v", err)
		}
		if got != "prod-token" {
			t.Errorf("Resolve() = %q, want prod-token", got)
		}
	}

	if resolver.calls["production"] != 1 {
		t.Errorf(
			"production key resolved %d times, want 1 (cached)",
			resolver.calls["production"],
		)
	}
	if resolver.calls["shared"] != 0 {
		t.Errorf(
			"shared key resolved %d times, want 0 (never referenced)",
			resolver.calls["shared"],
		)
	}
}

// TestManagerResolverMissingFileIsEmpty verifies an absent secrets file is
// optional and produces a resolver that reports references as dangling when
// revealed.
func TestManagerResolverMissingFileIsEmpty(t *testing.T) {
	t.Parallel()

	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, filepath.Join(t.TempDir(), "nope.yaml")),
		Cipher:            newTestCipher(t),
		PrivateKeyService: newPrivateKeyTestService(),
	})
	if err != nil {
		t.Fatalf("New() absent: %v", err)
	}
	r, err := manager.OpenResolver(true)
	if err != nil {
		t.Fatalf("OpenResolver() absent: %v", err)
	}
	if _, err := r.Resolve("secret://any/thing", ""); err == nil {
		t.Error("expected a missing-file resolver to reject a revealed reference")
	}
}

// TestManagerResolverMalformed verifies a malformed secrets file is an error.
func TestManagerResolverMalformed(t *testing.T) {
	t.Parallel()

	storePath := writeStore(t, "{")
	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, storePath),
		Cipher:            newTestCipher(t),
		PrivateKeyService: newPrivateKeyTestService(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if _, err := manager.OpenResolver(false); err == nil {
		t.Error("expected OpenResolver() to reject a malformed secrets file")
	}
}

// TestResolveMasked verifies plain values pass through and references mask to
// their canonical form, covering explicit groups, the shared group, a reference
// with no stored entry, and the backslash escape hatch.
func TestResolveMasked(t *testing.T) {
	t.Parallel()

	r := newMaskingResolver()
	tests := []struct {
		name, value, env, want string
	}{
		{"plain value", "localhost", "production", "localhost"},
		{"url passes through", "postgres://u@h/db", "production", "postgres://u@h/db"},
		{
			"explicit group",
			"secret://production/postgres_password", "dev",
			"secret://production/postgres_password",
		},
		{
			"shared group",
			"secret://shared/api_key", "production",
			"secret://shared/api_key",
		},
		{
			"dangling reference still masks",
			"secret://ghost/missing", "production",
			"secret://ghost/missing",
		},
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

// TestResolveMaskGroupCaseInsensitive verifies stored and referenced group names
// share the document-wide case-insensitive identity policy while masking.
func TestResolveMaskGroupCaseInsensitive(t *testing.T) {
	t.Parallel()

	path := writeStore(t, "secrets:\n  Production:\n    token: value\n")
	manager, err := NewService(ServiceParams{
		Repository:        newTestStore(t, path),
		Cipher:            newTestCipher(t),
		PrivateKeyService: newPrivateKeyTestService(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	r, err := manager.OpenResolver(false)
	if err != nil {
		t.Fatalf("OpenResolver() error = %v", err)
	}

	for _, ref := range []string{
		"secret://production/token",
		"secret://PRODUCTION/token",
	} {
		got, err := r.Resolve(ref, "")
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", ref, err)
		}
		if want := "secret://production/token"; got != want {
			t.Errorf("Resolve(%q) = %q, want %q", ref, got, want)
		}
	}
}

// TestResolveMaskedRejectsMalformedReference verifies a masked resolver still
// rejects references whose grammar is invalid, even though it tolerates a
// well-formed reference with no stored entry.
func TestResolveMaskedRejectsMalformedReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, value, env string
	}{
		{"group-less key", "secret://postgres_password", "production"},
		{"too many slashes", "secret://a/b/c", "production"},
		{"empty reference", "secret://", "production"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newMaskingResolver()
			if _, err := r.Resolve(tt.value, tt.env); err == nil {
				t.Errorf("Resolve(%q) expected error", tt.value)
			}
		})
	}
}

// TestManagerResolverRevealDanglingFails verifies a revealing resolver rejects a
// well-formed reference with no stored entry, since revealing materializes
// plaintext that does not exist.
func TestManagerResolverRevealDanglingFails(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, fixedPrivateKeyResolver{})
	r, err := manager.OpenResolver(true)
	if err != nil {
		t.Fatalf("OpenResolver(): %v", err)
	}
	if _, err := r.Resolve("secret://production/missing", ""); err == nil {
		t.Fatal("revealing a dangling reference should fail")
	}
}

// TestResolveEmptyReferenceMessage verifies an empty reference reports the empty
// fault rather than a generic malformed-reference error.
func TestResolveEmptyReferenceMessage(t *testing.T) {
	t.Parallel()

	r := newMaskingResolver()
	_, err := r.Resolve("secret://", "production")
	if err == nil {
		t.Fatal("expected error for empty reference")
	}
	if !strings.Contains(err.Error(), "empty secret reference") {
		t.Errorf("error = %v, want it to mention an empty secret reference", err)
	}
}
