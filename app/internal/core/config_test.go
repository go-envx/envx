package core

import (
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/test/fixtures"
)

// TestResolveWorkspace verifies each feature config is derived from a fixture
// manifest.
func TestResolveWorkspace(t *testing.T) {
	t.Parallel()

	res, err := resolveWorkspace(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("resolveWorkspace: %v", err)
	}
	if filepath.Base(res.secrets.SecretsPath) != "secrets.yaml" {
		t.Errorf("SecretsPath = %q, want .../secrets.yaml", res.secrets.SecretsPath)
	}
	wantKeys := filepath.Join(filepath.Dir(res.secrets.SecretsPath), "envx.keys")
	if res.secrets.KeysPath != wantKeys {
		t.Errorf("KeysPath = %q, want %q", res.secrets.KeysPath, wantKeys)
	}
	if res.secrets.Cipher != string(cipher.Age) {
		t.Errorf("Cipher = %q, want %q", res.secrets.Cipher, cipher.Age)
	}
	if len(res.env.Projects) == 0 || len(res.pack.Projects) == 0 ||
		len(res.validate.Projects) == 0 {
		t.Errorf("projects missing: env=%d pack=%d validate=%d",
			len(res.env.Projects), len(res.pack.Projects), len(res.validate.Projects))
	}
}

// TestEnvServiceResolvesFixtureProject verifies the composed env service
// resolves a known fixture project end to end.
func TestEnvServiceResolvesFixtureProject(t *testing.T) {
	t.Parallel()

	service, err := NewApp().EnvService(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("EnvService: %v", err)
	}
	entry, err := service.Get(env.GetParams{Project: "api-core", Key: "APP_NAME"})
	if err != nil {
		t.Fatalf("Get APP_NAME: %v", err)
	}
	if entry.Value != "api-core" {
		t.Errorf("APP_NAME = %q, want api-core", entry.Value)
	}
}

// TestEnvServiceMasksSecretReference verifies read commands mask secret
// references by default without decrypting or requiring a private key.
func TestEnvServiceMasksSecretReference(t *testing.T) {
	t.Parallel()

	service, err := NewApp().EnvService(fixtures.Manifest("resolve/secret-reference"))
	if err != nil {
		t.Fatalf("EnvService: %v", err)
	}
	password, err := service.Get(env.GetParams{Project: "api", Key: "PASSWORD"})
	if err != nil {
		t.Fatalf("Get PASSWORD: %v", err)
	}
	if password.Value != "secret://development/api_key" {
		t.Errorf("PASSWORD = %q, want masked development reference", password.Value)
	}
	token, err := service.Get(env.GetParams{Project: "api", Key: "TOKEN"})
	if err != nil {
		t.Fatalf("Get TOKEN: %v", err)
	}
	if token.Value != "secret://shared/token" {
		t.Errorf("TOKEN = %q, want masked shared reference", token.Value)
	}
}

// TestEnvServiceIgnoresSecretsStoreAtConstruction verifies composing the env
// service never reads a malformed secrets store.
func TestEnvServiceIgnoresSecretsStoreAtConstruction(t *testing.T) {
	t.Parallel()

	_, err := NewApp().EnvService(fixtures.Manifest("resolve/global-ignores-store"))
	if err != nil {
		t.Fatalf("EnvService: %v", err)
	}
}

// TestEnvServiceDanglingSecretReference verifies a reference with no matching
// store entry masks to its canonical form for a default read but fails loudly
// once materialized.
func TestEnvServiceDanglingSecretReference(t *testing.T) {
	t.Parallel()

	service, err := NewApp().EnvService(fixtures.Manifest("resolve/dangling-reference"))
	if err != nil {
		t.Fatalf("EnvService: %v", err)
	}
	entry, err := service.Get(env.GetParams{Project: "api", Key: "PASSWORD"})
	if err != nil {
		t.Fatalf("Get masked: %v", err)
	}
	if entry.Value != "secret://development/missing" {
		t.Errorf("PASSWORD = %q, want the masked reference", entry.Value)
	}
	if _, err := service.Materialize(env.MaterializeParams{Project: "api"}); err == nil {
		t.Fatal("expected dangling reference error when materialized")
	}
}
