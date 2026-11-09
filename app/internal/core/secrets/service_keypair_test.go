package secrets_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
)

// TestGenerateKeypair verifies generation retains the private key, reports
// locations, and refuses to replace an existing identity.
func TestGenerateKeypair(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	res, err := f.service.GenerateKeypair("API")
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	if res.Keypair.Group != "api" ||
		res.Keypair.PrivateKeyStatus != secrets.PrivateKeyValid {
		t.Errorf("Keypair = %+v", res.Keypair)
	}
	if res.PublicKeyLocation != "mem://secrets" || res.PrivateKeyLocation != "mem://keys" {
		t.Errorf("locations = %q, %q", res.PublicKeyLocation, res.PrivateKeyLocation)
	}
	if _, ok := f.keys.byPublic["api"][res.Keypair.PublicKey]; !ok {
		t.Error("private key was not retained under its public key")
	}

	_, err = f.service.GenerateKeypair("api")
	mustErrorIs(t, err, secrets.ErrKeypairExists)
}

// TestGenerateKeypairRetainFailureCommitsNothing verifies no public key is
// committed when the private key cannot be retained.
func TestGenerateKeypairRetainFailureCommitsNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys.retainErr = errors.New("disk full")

	if _, err := f.service.GenerateKeypair("api"); err == nil {
		t.Fatal("GenerateKeypair succeeded, want error")
	}
	if f.store.commits != 0 {
		t.Errorf("committed public key after retention failure")
	}
}

// TestInspectKeypair verifies valid, mismatched, and missing private keys map
// to their statuses.
func TestInspectKeypair(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	publicKey := f.generate(t, "api")
	other := f.generate(t, "other")

	meta, err := f.service.InspectKeypair("api")
	if err != nil || meta.PrivateKeyStatus != secrets.PrivateKeyValid {
		t.Errorf("valid key: %+v, %v", meta, err)
	}

	f.keys.byPublic["api"][publicKey] = f.keys.byPublic["other"][other]
	meta, err = f.service.InspectKeypair("api")
	if err != nil || meta.PrivateKeyStatus != secrets.PrivateKeyInvalid {
		t.Errorf("mismatched key: %+v, %v", meta, err)
	}

	delete(f.keys.byPublic, "api")
	meta, err = f.service.InspectKeypair("api")
	if err != nil || meta.PrivateKeyStatus != secrets.PrivateKeyNotAvailable {
		t.Errorf("missing key: %+v, %v", meta, err)
	}

	_, err = f.service.InspectKeypair("missing")
	mustErrorIs(t, err, secrets.ErrGroupNotFound)
}
