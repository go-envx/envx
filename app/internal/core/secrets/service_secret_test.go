package secrets_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
)

// TestSetGetRoundTrip verifies Set stores an age envelope under the normalized
// group and Get decrypts it back.
func TestSetGetRoundTrip(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.generate(t, "API")

	res, err := f.service.SetSecret("API", "TOKEN", plaintextOf("s3cr3t"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := secrets.Reference{Group: "api", Key: "TOKEN"}
	if res.Secret != want || res.Location != "mem://secrets" {
		t.Errorf("Set result = %+v", res)
	}
	if stored := f.stored("api", "TOKEN"); !strings.HasPrefix(stored, "encrypted-age:") {
		t.Errorf("stored value = %q, want age envelope", stored)
	}

	got, err := f.service.GetSecret("api", "TOKEN")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Value != "s3cr3t" {
		t.Errorf("Get value = %q, want s3cr3t", got.Value)
	}
}

// TestSetRejectsInvalidInput verifies Set rejects missing keypairs, empty
// plaintext, and malformed names.
func TestSetRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.generate(t, "api")

	tests := []struct {
		name      string
		group     string
		key       string
		plaintext string
		target    error
	}{
		{"missing public key", "web", "K", "v", secrets.ErrGroupNotFound},
		{"empty plaintext", "api", "K", "", nil},
		{"invalid group", "a b", "K", "v", nil},
		{"invalid key", "api", "a/b", "v", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.service.SetSecret(tt.group, tt.key, plaintextOf(tt.plaintext))
			if err == nil {
				t.Fatal("Set succeeded, want error")
			}
			if tt.target != nil && !errors.Is(err, tt.target) {
				t.Errorf("error = %v, want %v", err, tt.target)
			}
		})
	}
}

// TestSetReadsPlaintextAfterValidation verifies the plaintext source is not
// called when the reference is invalid or the group has no public key.
func TestSetReadsPlaintextAfterValidation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	called := false
	source := func() (string, error) {
		called = true
		return "v", nil
	}
	if _, err := f.service.SetSecret("a b", "K", source); err == nil {
		t.Error("Set with invalid group succeeded")
	}
	_, err := f.service.SetSecret("web", "K", source)
	mustErrorIs(t, err, secrets.ErrGroupNotFound)
	if called {
		t.Error("plaintext source was called before validation passed")
	}
}

// TestGetFailures verifies Get reports missing and unencrypted values without
// resolving a private key.
func TestGetFailures(t *testing.T) {
	t.Parallel()
	f := newFixture(t, secrets.Group{
		Name:   "legacy",
		Values: map[string]string{"PLAIN": "value"},
	})
	f.generate(t, "api")

	_, err := f.service.GetSecret("api", "MISSING")
	mustErrorIs(t, err, secrets.ErrSecretNotFound)

	_, err = f.service.GetSecret("legacy", "PLAIN")
	if err == nil || !strings.Contains(err.Error(), "not encrypted") {
		t.Errorf("Get plaintext error = %v, want not encrypted", err)
	}
	if f.keys.resolves != 0 {
		t.Errorf("resolved private keys %d times, want 0", f.keys.resolves)
	}
}

// TestDeleteKeepsPublicKey verifies Delete removes only the value and rejects
// a second delete.
func TestDeleteKeepsPublicKey(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	publicKey := f.generate(t, "api")
	f.set(t, "api", "TOKEN", "v")

	if _, err := f.service.DeleteSecret("api", "TOKEN"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	group := f.store.groups["api"]
	if len(group.Values) != 0 || group.PublicKey != publicKey {
		t.Errorf("group after delete = %+v", group)
	}

	_, err := f.service.DeleteSecret("api", "TOKEN")
	mustErrorIs(t, err, secrets.ErrSecretNotFound)
}
