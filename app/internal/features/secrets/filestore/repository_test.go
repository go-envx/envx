package filestore

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/features/secrets"
)

func TestRepositoryCRUD(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.yaml")

	repo, err := New(Params{
		Path:          path,
		DefaultIndent: 2,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// 1. Initial store is empty
	key, found, err := repo.GetPublicKey("prod")
	if err != nil {
		t.Fatalf("GetPublicKey() error = %v", err)
	}
	if found || key != "" {
		t.Fatalf("expected not found, got found=%v key=%q", found, key)
	}

	keypairs, err := repo.ListKeypairs()
	if err != nil {
		t.Fatalf("ListKeypairs() error = %v", err)
	}
	if len(keypairs) != 0 {
		t.Fatalf("expected 0 keypairs, got %d", len(keypairs))
	}

	sec, found, err := repo.GetSecret("prod", "api_key")
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}
	if found {
		t.Fatalf("expected secret not found, got %v", sec)
	}

	// 2. Set public key and retrieve
	if err := repo.SetPublicKey("prod", "age1publickeytest"); err != nil {
		t.Fatalf("SetPublicKey() error = %v", err)
	}
	key, found, err = repo.GetPublicKey("prod")
	if err != nil {
		t.Fatalf("GetPublicKey() error = %v", err)
	}
	if !found || key != "age1publickeytest" {
		t.Fatalf("expected found=true key=age1publickeytest, got found=%v key=%q", found, key)
	}

	keypairs, err = repo.ListKeypairs()
	if err != nil {
		t.Fatalf("ListKeypairs() error = %v", err)
	}
	if len(keypairs) != 1 ||
		keypairs[0].Group != "prod" ||
		keypairs[0].PublicKey != "age1publickeytest" {
		t.Fatalf("unexpected keypairs: %+v", keypairs)
	}

	// 3. Set secrets
	rec1 := secrets.SecretRecord{
		Group:      "prod",
		Key:        "api_key",
		Ciphertext: "encrypted-age:YWJj",
	}
	if err := repo.SetSecret(rec1); err != nil {
		t.Fatalf("SetSecret() error = %v", err)
	}

	gotSec, found, err := repo.GetSecret("prod", "api_key")
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}
	if !found || gotSec.Ciphertext != "encrypted-age:YWJj" || gotSec.Algorithm() != "age" {
		t.Fatalf("unexpected secret: %+v", gotSec)
	}

	// 4. Batch set secrets
	rec2 := secrets.SecretRecord{
		Group:      "prod",
		Key:        "db_pass",
		Ciphertext: "encrypted-age:ZGVm",
	}
	rec3 := secrets.SecretRecord{
		Group:      "staging",
		Key:        "token",
		Ciphertext: "plaintext-token",
	}
	if err := repo.SetSecrets([]secrets.SecretRecord{rec2, rec3}); err != nil {
		t.Fatalf("SetSecrets() error = %v", err)
	}

	allSecs, err := repo.ListSecrets()
	if err != nil {
		t.Fatalf("ListSecrets() error = %v", err)
	}
	if len(allSecs) != 3 {
		t.Fatalf("expected 3 secrets, got %d: %+v", len(allSecs), allSecs)
	}

	// 5. Delete secret
	if err := repo.DeleteSecret("prod", "api_key"); err != nil {
		t.Fatalf("DeleteSecret() error = %v", err)
	}
	_, found, err = repo.GetSecret("prod", "api_key")
	if err != nil {
		t.Fatalf("GetSecret() after delete error = %v", err)
	}
	if found {
		t.Fatal("expected secret to be deleted")
	}

	// 6. Delete missing secret returns error wrapping ErrSecretNotFound
	err = repo.DeleteSecret("prod", "api_key")
	if err == nil {
		t.Fatal("expected error deleting missing secret, got nil")
	}
	if !errors.Is(err, secrets.ErrSecretNotFound) {
		t.Fatalf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestRepositoryParamsValidation(t *testing.T) {
	t.Parallel()

	if _, err := New(Params{}); err == nil {
		t.Fatal("expected error with empty path, got nil")
	}

	if _, err := New(Params{Path: "   "}); err == nil {
		t.Fatal("expected error with whitespace path, got nil")
	}
}
