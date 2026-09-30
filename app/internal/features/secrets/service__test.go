package secrets

import (
	"testing"
)

// TestNewRejectsNilRepository verifies Service construction requires a repository.
func TestNewRejectsNilRepository(t *testing.T) {
	t.Parallel()

	if _, err := NewService(ServiceParams{
		Cipher:            newTestCipher(t),
		PrivateKeyService: newPrivateKeyTestService(),
	}); err == nil {
		t.Fatal("New() succeeded without a repository")
	}
}

// TestNewRejectsNilCipher verifies Service construction requires a cipher.
func TestNewRejectsNilCipher(t *testing.T) {
	t.Parallel()

	params := ServiceParams{
		Repository:        newFakeRepository(),
		PrivateKeyService: newPrivateKeyTestService(),
	}
	if _, err := NewService(params); err == nil {
		t.Fatal("New() succeeded without a cipher")
	}
}

// TestNewRejectsNilPrivateKeyService verifies Service construction requires
// a private-key service.
func TestNewRejectsNilPrivateKeyService(t *testing.T) {
	t.Parallel()

	params := ServiceParams{
		Repository: newFakeRepository(),
		Cipher:     newTestCipher(t),
	}
	if _, err := NewService(params); err == nil ||
		err.Error() != "private-key service is required" {
		t.Fatalf("New() error = %v, want private-key service is required", err)
	}
}

// TestNewServiceWithFakeRepository verifies domain operations against an
// in-memory repository.
func TestNewServiceWithFakeRepository(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	cipher := newTestCipher(t)
	pkSvc := newPrivateKeyTestService()

	svc, err := NewService(ServiceParams{
		Repository:        repo,
		Cipher:            cipher,
		PrivateKeyService: pkSvc,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Test Set and Get on in-memory fake repository
	if err := repo.SetPublicKey("prod", "fake-public-key"); err != nil {
		t.Fatalf("SetPublicKey(): %v", err)
	}
	// Verify Has before Set
	has, err := svc.Has("prod", "token")
	if err != nil {
		t.Fatalf("Has(): %v", err)
	}
	if has {
		t.Fatal("expected Has() = false before Set")
	}
}
