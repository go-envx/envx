package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/pack"
	"github.com/go-envx/envx/app/internal/features/validate"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/test/fixtures"
)

// newTestFactory constructs an AppFactory or fails the test.
func newTestFactory(t *testing.T) *AppFactory {
	t.Helper()
	factory, err := NewAppFactory()
	if err != nil {
		t.Fatalf("NewAppFactory(): %v", err)
	}
	return factory
}

// TestComposeAppConfigDerivesEverySlice verifies each feature config is derived
// from a fixture manifest.
func TestComposeAppConfigDerivesEverySlice(t *testing.T) {
	t.Parallel()

	config, err := composeAppConfig(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("composeAppConfig(): %v", err)
	}
	if filepath.Base(config.Secrets.SecretsPath) != "secrets.yaml" {
		t.Errorf("SecretsPath = %q, want .../secrets.yaml", config.Secrets.SecretsPath)
	}
	wantKeys := filepath.Join(filepath.Dir(config.Secrets.SecretsPath), "envx.keys")
	if config.Secrets.KeysPath != wantKeys {
		t.Errorf("KeysPath = %q, want %q", config.Secrets.KeysPath, wantKeys)
	}
	if config.Secrets.Cipher != string(cipher.Age) {
		t.Errorf("Cipher = %q, want %q", config.Secrets.Cipher, cipher.Age)
	}
	if len(config.Env.Projects) == 0 || len(config.Pack.Projects) == 0 ||
		len(config.Validate.Projects) == 0 {
		t.Errorf("projects missing: env=%d pack=%d validate=%d",
			len(config.Env.Projects), len(config.Pack.Projects),
			len(config.Validate.Projects))
	}
}

// TestFactoryAccessorsReturnWiredServices verifies every workspace-bound accessor
// returns a service for a fixture workspace.
func TestFactoryAccessorsReturnWiredServices(t *testing.T) {
	t.Parallel()

	factory := newTestFactory(t)
	manifest := fixtures.Manifest("basic")

	if service, err := factory.EnvService(manifest); err != nil || service == nil {
		t.Errorf("EnvService() = %v, %v, want a service", service, err)
	}
	if service, err := factory.SecretsService(manifest); err != nil || service == nil {
		t.Errorf("SecretsService() = %v, %v, want a service", service, err)
	}
	if service, err := factory.PackService(manifest); err != nil || service == nil {
		t.Errorf("PackService() = %v, %v, want a service", service, err)
	}
	if service, err := factory.ValidateService(manifest); err != nil || service == nil {
		t.Errorf("ValidateService() = %v, %v, want a service", service, err)
	}
}

// TestFactoryMemoizesAssemblyPerConfigPath verifies repeated accessor calls for
// one config path share a single assembly, and distinct paths do not.
func TestFactoryMemoizesAssemblyPerConfigPath(t *testing.T) {
	t.Parallel()

	factory := newTestFactory(t)
	basic := fixtures.Manifest("basic")

	first, err := factory.EnvService(basic)
	if err != nil {
		t.Fatalf("EnvService(): %v", err)
	}
	second, err := factory.EnvService(basic)
	if err != nil {
		t.Fatalf("EnvService() again: %v", err)
	}
	if first != second {
		t.Error("EnvService() built a second env service for the same config path")
	}

	assembled, err := factory.workspaceServices(basic)
	if err != nil {
		t.Fatalf("workspaceServices(): %v", err)
	}
	if assembled.Env != first {
		t.Error("accessors for one config path do not share one assembly")
	}

	other, err := factory.EnvService(fixtures.Manifest("resolve/secret-reference"))
	if err != nil {
		t.Fatalf("EnvService(other): %v", err)
	}
	if other == first {
		t.Error("distinct config paths shared an env service")
	}
}

// TestFactoryWorkspaceFreeServicesNeedNoManifest verifies workspace-free
// accessors succeed without any manifest present.
func TestFactoryWorkspaceFreeServicesNeedNoManifest(t *testing.T) {
	t.Chdir(t.TempDir())

	factory := newTestFactory(t)

	if service, err := factory.ScaffoldService(); err != nil || service == nil {
		t.Errorf("ScaffoldService() = %v, %v, want a service", service, err)
	}
	if service, err := factory.RunnerService(); err != nil || service == nil {
		t.Errorf("RunnerService() = %v, %v, want a service", service, err)
	}
	if service, err := factory.EmitService(); err != nil || service == nil {
		t.Errorf("EmitService() = %v, %v, want a service", service, err)
	}
}

// TestFactoryLoadErrorIsConsistentAcrossAccessors verifies a missing manifest
// fails every workspace-bound accessor with the same error.
func TestFactoryLoadErrorIsConsistentAcrossAccessors(t *testing.T) {
	t.Parallel()

	factory := newTestFactory(t)
	missing := filepath.Join(t.TempDir(), "envx.yaml")

	_, envErr := factory.EnvService(missing)
	_, secretsErr := factory.SecretsService(missing)
	_, packErr := factory.PackService(missing)
	_, validateErr := factory.ValidateService(missing)

	if envErr == nil {
		t.Fatal("EnvService() succeeded without a manifest")
	}
	for name, err := range map[string]error{
		"SecretsService":  secretsErr,
		"PackService":     packErr,
		"ValidateService": validateErr,
	} {
		if !errors.Is(err, envErr) {
			t.Errorf("%s() error = %v, want %v", name, err, envErr)
		}
	}
}

// TestFactoryValidateServiceDiagnosesEveryProject verifies the composed validate
// service resolves the workspace's projects and environments end to end.
func TestFactoryValidateServiceDiagnosesEveryProject(t *testing.T) {
	t.Parallel()

	service, err := newTestFactory(t).ValidateService(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("ValidateService(): %v", err)
	}

	report, err := service.Validate(validate.ValidateParams{})
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if len(report.Findings) != 0 || report.Failed {
		t.Errorf("report = %+v, want a clean pass for the basic fixture", report)
	}
}

// TestFactoryPackServiceWritesBundle verifies the composed pack service copies the
// workspace into a bundle.
func TestFactoryPackServiceWritesBundle(t *testing.T) {
	t.Parallel()

	service, err := newTestFactory(t).PackService(fixtures.Manifest("basic"))
	if err != nil {
		t.Fatalf("PackService(): %v", err)
	}

	out := filepath.Join(t.TempDir(), "dist")
	result, err := service.Pack(pack.PackParams{OutDir: out})
	if err != nil {
		t.Fatalf("Pack(): %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, result.ManifestFile)); err != nil {
		t.Errorf("bundle manifest not written: %v", err)
	}
}

// TestFactoryEnvServiceResolvesFixtureProject verifies the composed env service
// resolves a known fixture project end to end.
func TestFactoryEnvServiceResolvesFixtureProject(t *testing.T) {
	t.Parallel()

	service, err := newTestFactory(t).EnvService(fixtures.Manifest("basic"))
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

// TestFactoryEnvServiceMasksSecretReference verifies read commands mask secret
// references by default without decrypting or requiring a private key.
func TestFactoryEnvServiceMasksSecretReference(t *testing.T) {
	t.Parallel()

	service, err := newTestFactory(t).EnvService(
		fixtures.Manifest("resolve/secret-reference"),
	)
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

// TestFactoryEnvServiceIgnoresSecretsStoreAtConstruction verifies composing the
// env service never reads a malformed secrets store.
func TestFactoryEnvServiceIgnoresSecretsStoreAtConstruction(t *testing.T) {
	t.Parallel()

	_, err := newTestFactory(t).EnvService(
		fixtures.Manifest("resolve/global-ignores-store"),
	)
	if err != nil {
		t.Fatalf("EnvService: %v", err)
	}
}

// TestFactoryEnvServiceDanglingSecretReference verifies a reference with no
// matching store entry masks to its canonical form for a default read but fails
// loudly once materialized.
func TestFactoryEnvServiceDanglingSecretReference(t *testing.T) {
	t.Parallel()

	service, err := newTestFactory(t).EnvService(
		fixtures.Manifest("resolve/dangling-reference"),
	)
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
