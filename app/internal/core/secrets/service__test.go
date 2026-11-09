package secrets_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/lib/crypt/age"
)

// fixture bundles a service with its fakes.
type fixture struct {
	// store is the in-memory store behind service.
	store *memStore
	// keys is the in-memory keyring behind service.
	keys *memKeys
	// cipher is the real age engine.
	cipher age.Cipher
	// service is the service under test.
	service *secrets.Service
}

// newFixture returns a service over fresh fakes and the age cipher.
func newFixture(t *testing.T, groups ...secrets.Group) *fixture {
	t.Helper()
	f := &fixture{store: newMemStore(groups...), keys: newMemKeys()}
	service, err := secrets.New(f.store, f.keys, f.cipher)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	f.service = service
	return f
}

// generate creates a keypair for group through the service.
func (f *fixture) generate(t *testing.T, group string) string {
	t.Helper()
	res, err := f.service.GenerateKeypair(group)
	if err != nil {
		t.Fatalf("GenerateKeypair(%q): %v", group, err)
	}
	return res.Keypair.PublicKey
}

// set stores plaintext through the service.
func (f *fixture) set(t *testing.T, group, key, plaintext string) {
	t.Helper()
	ref := secrets.Reference{Group: group, Key: key}
	if _, err := f.service.Set(ref, plaintext); err != nil {
		t.Fatalf("Set(%v): %v", ref, err)
	}
}

// stored returns the raw stored value.
func (f *fixture) stored(group, key string) string {
	return f.store.groups[group].Values[key]
}

// mustErrorIs fails the test unless err wraps target.
func mustErrorIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want %v", err, target)
	}
}

// TestNewRequiresDependencies verifies New rejects each missing dependency.
func TestNewRequiresDependencies(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if _, err := secrets.New(nil, f.keys, f.cipher); err == nil {
		t.Error("New without store succeeded")
	}
	if _, err := secrets.New(f.store, nil, f.cipher); err == nil {
		t.Error("New without keys succeeded")
	}
	if _, err := secrets.New(f.store, f.keys, nil); err == nil {
		t.Error("New without cipher succeeded")
	}
}
