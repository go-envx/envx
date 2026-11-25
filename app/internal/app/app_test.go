package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/scaffold"
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/validate"
	"github.com/go-envx/envx/app/internal/core/value"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/testutil"
)

// minimalWorkspace is the read-only composition fixture.
const minimalWorkspace = "testdata/minimal-workspace"

// countingFactory returns a factory whose environment serves environment and
// counts reads.
func countingFactory(environment ...string) (Factory, *atomic.Int32) {
	captures := &atomic.Int32{}
	return Factory{Environ: func() []string {
		captures.Add(1)
		return environment
	}}, captures
}

// writeFile replaces the file at path with data.
func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestConstructorsAreLazy verifies workspace constructors neither capture the
// host environment nor touch secret files.
func TestConstructorsAreLazy(t *testing.T) {
	t.Parallel()

	workspaceDir := testutil.CopyDir(t, minimalWorkspace)
	writeFile(t, filepath.Join(workspaceDir, "envx.local.env"), "A=\"unterminated\n")
	f, captures := countingFactory()
	if _, err := f.NewEnvService(workspaceDir); err != nil {
		t.Fatalf("NewEnvService: %v", err)
	}
	if _, err := f.NewSecretsService(workspaceDir); err != nil {
		t.Fatalf("NewSecretsService: %v", err)
	}
	if _, err := f.NewPackService(workspaceDir); err != nil {
		t.Fatalf("NewPackService: %v", err)
	}
	if _, err := f.NewValidateService(workspaceDir); err != nil {
		t.Fatalf("NewValidateService: %v", err)
	}
	if _, err := f.NewEmitService(workspaceDir); err != nil {
		t.Fatalf("NewEmitService: %v", err)
	}
	if got := captures.Load(); got != 0 {
		t.Errorf("environment read %d times, want 0", got)
	}
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Errorf("workspace has %d entries after construction, want 3", len(entries))
	}
}

// TestNewEnvServiceReadsHostOnce verifies a revealed read decrypts through
// the store and keyring, that the resolver reads private-key overrides from the
// service's host lookup, and that the host is read once per service.
func TestNewEnvServiceReadsHostOnce(t *testing.T) {
	t.Parallel()

	workspaceDir := testutil.CopyDir(t, minimalWorkspace)
	f, _ := countingFactory()
	manager, err := f.NewSecretsService(workspaceDir)
	if err != nil {
		t.Fatalf("NewSecretsService: %v", err)
	}
	if _, err := manager.GenerateKeypair("app"); err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	plaintext := func() (string, error) { return "s3cr3t", nil }
	if _, err := manager.SetSecret("app", "token", plaintext); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	getParams := env.GetParams{
		Project: "app",
		Key:     "token",
		Reveal:  true,
	}

	f, captures := countingFactory()
	service, err := f.NewEnvService(workspaceDir)
	if err != nil {
		t.Fatalf("NewEnvService: %v", err)
	}
	got, err := service.Get(getParams)
	if err != nil || got.Value != "s3cr3t" {
		t.Errorf("Get = %q, %v; want s3cr3t", got.Value, err)
	}
	if _, err := service.Get(getParams); err != nil {
		t.Errorf("second Get: %v", err)
	}
	if count := captures.Load(); count != 1 {
		t.Errorf("environment read %d times, want 1", count)
	}

	f, _ = countingFactory("ENVX_PRIVATE_KEY_APP=")
	service, err = f.NewEnvService(workspaceDir)
	if err != nil {
		t.Fatalf("NewEnvService: %v", err)
	}
	if _, err := service.Get(getParams); !errors.Is(err, secrets.ErrInvalidKey) {
		t.Errorf("Get with empty key override error = %v, want ErrInvalidKey", err)
	}
	getParams.Reveal = false
	got, err = service.Get(getParams)
	if err != nil || got.Value != "secret://app/token" {
		t.Errorf("masked Get = %q, %v; want secret://app/token", got.Value, err)
	}
}

// localEnvWorkspace returns a workspace whose only private key lives in a local
// env file next to it, with a secret and a {{REGION}} reference to resolve.
func localEnvWorkspace(t *testing.T) (dir, key string) {
	t.Helper()
	dir = testutil.CopyDir(t, minimalWorkspace)
	f, _ := countingFactory()
	manager, err := f.NewSecretsService(dir)
	if err != nil {
		t.Fatalf("NewSecretsService: %v", err)
	}
	if _, err := manager.GenerateKeypair("app"); err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	plaintext := func() (string, error) { return "s3cr3t", nil }
	if _, err := manager.SetSecret("app", "token", plaintext); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	keysPath := filepath.Join(dir, "envx.local.keys")
	//nolint:gosec // test-local path
	data, err := os.ReadFile(keysPath)
	if err != nil {
		t.Fatal(err)
	}
	_, key, _ = strings.Cut(strings.TrimSpace(string(data)), "=")
	if err := os.Remove(keysPath); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "env", "app.yaml"),
		"greeting: hello\ntoken: secret://app/token\nurl: \"api.{{REGION}}\"\n")
	return dir, key
}

// TestLocalEnvFileFeedsGet verifies the local env file supplies {{VAR}}
// values and the private key, and the process environment wins over it.
func TestLocalEnvFileFeedsGet(t *testing.T) {
	t.Parallel()

	dir, key := localEnvWorkspace(t)
	writeFile(t, filepath.Join(dir, "envx.local.env"),
		"REGION=eu\nENVX_PRIVATE_KEY_APP='"+key+"'\n")

	f, _ := countingFactory()
	service, err := f.NewEnvService(dir)
	if err != nil {
		t.Fatalf("NewEnvService: %v", err)
	}
	for param, want := range map[string]string{"url": "api.eu", "token": "s3cr3t"} {
		got, err := service.Get(env.GetParams{Project: "app", Key: param, Reveal: true})
		if err != nil || got.Value != want {
			t.Errorf("Get %s = %q, %v; want %q", param, got.Value, err, want)
		}
	}

	f, _ = countingFactory("REGION=us")
	service, err = f.NewEnvService(dir)
	if err != nil {
		t.Fatalf("NewEnvService: %v", err)
	}
	got, err := service.Get(env.GetParams{Project: "app", Key: "url", Reveal: true})
	if err != nil || got.Value != "api.us" {
		t.Errorf("Get url = %q, %v; want api.us", got.Value, err)
	}
}

// TestLocalEnvVarSelectsFile verifies ENVX_LOCAL_ENV selects a file outside the
// workspace.
func TestLocalEnvVarSelectsFile(t *testing.T) {
	t.Parallel()

	dir, key := localEnvWorkspace(t)
	outside := filepath.Join(t.TempDir(), "machine.env")
	writeFile(t, outside, "REGION=ap\nENVX_PRIVATE_KEY_APP='"+key+"'\n")

	f, _ := countingFactory("ENVX_LOCAL_ENV=" + outside)
	service, err := f.NewEnvService(dir)
	if err != nil {
		t.Fatalf("NewEnvService: %v", err)
	}
	for param, want := range map[string]string{"url": "api.ap", "token": "s3cr3t"} {
		got, err := service.Get(env.GetParams{Project: "app", Key: param, Reveal: true})
		if err != nil || got.Value != want {
			t.Errorf("Get %s = %q, %v; want %q", param, got.Value, err, want)
		}
	}
}

// TestNewEnvServiceRegistersDeclaredProviders verifies every declared provider
// masks its references without contacting the provider.
func TestNewEnvServiceRegistersDeclaredProviders(t *testing.T) {
	t.Parallel()

	workspaceDir := testutil.CopyDir(t, minimalWorkspace)
	writeFile(t, filepath.Join(workspaceDir, "envx.yaml"), strings.Join([]string{
		"environments: [dev]",
		"secrets:",
		"  providers:",
		"    op: {}",
		"    bws: {}",
		"    passwordstate: {}",
		"projects:",
		"  app:",
		"    includes: [env/app]",
		"",
	}, "\n"))
	writeFile(t, filepath.Join(workspaceDir, "env", "app.yaml"), strings.Join([]string{
		"one: op://vault/item/field",
		"two: bws://project/key",
		"three: passwordstate://42",
		"",
	}, "\n"))
	f, _ := countingFactory()
	service, err := f.NewEnvService(workspaceDir)
	if err != nil {
		t.Fatalf("NewEnvService: %v", err)
	}
	result, err := service.Explain(env.ExplainParams{Project: "app"})
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	entries := make(map[string]env.ExplainEntry)
	for _, entry := range result.Entries {
		entries[entry.Key] = entry
	}
	op := entries["ONE"].Evaluation
	if op.Kind != value.Secret || op.Value != "op://vault/item/field" ||
		!op.Diagnostic.IsZero() {
		t.Errorf("op evaluation = %+v, want masked secret", op)
	}
	bws := entries["TWO"].Evaluation
	if bws.Kind != value.Secret || bws.Value != "bws://project/key" ||
		!bws.Diagnostic.IsZero() {
		t.Errorf("bws evaluation = %+v, want masked secret", bws)
	}
	pws := entries["THREE"].Evaluation
	if pws.Kind != value.Secret || pws.Value != "passwordstate://42" ||
		!pws.Diagnostic.IsZero() {
		t.Errorf("passwordstate evaluation = %+v, want masked secret", pws)
	}
}

// TestNewValidateServiceOpensEnvOnlyForMergeChecks verifies store-only checks run
// without assembling the environment service.
func TestNewValidateServiceOpensEnvOnlyForMergeChecks(t *testing.T) {
	t.Parallel()

	workspaceDir := testutil.CopyDir(t, minimalWorkspace)
	writeFile(t, filepath.Join(workspaceDir, "env", "app.yaml"), "[unterminated\n")
	f, _ := countingFactory()
	service, err := f.NewValidateService(workspaceDir)
	if err != nil {
		t.Fatalf("NewValidateService: %v", err)
	}

	report, err := service.Validate(validate.ValidateParams{
		Selected: []diagnostic.Code{diagnostic.PublicKeyIsMissing},
	})
	if err != nil || report.Failed {
		t.Errorf("store-only Validate = %+v, %v; want pass", report, err)
	}
	if _, err := service.Validate(validate.ValidateParams{}); err == nil {
		t.Error("Validate of all checks succeeded over a malformed namespace")
	}
}

// TestNewValidateServiceChecksProvidersOffline verifies validate reports each
// referenced, unavailable provider once and malformed references per entry.
func TestNewValidateServiceChecksProvidersOffline(t *testing.T) {
	t.Parallel()

	workspaceDir := testutil.CopyDir(t, minimalWorkspace)
	writeFile(t, filepath.Join(workspaceDir, "envx.yaml"), strings.Join([]string{
		"environments: [dev, prod]",
		"secrets:",
		"  providers:",
		"    bws: {project: {dev: app}}",
		"    passwordstate: {}",
		"projects:",
		"  app:",
		"    includes: [env/app]",
		"",
	}, "\n"))
	writeFile(t, filepath.Join(workspaceDir, "env", "app.yaml"), strings.Join([]string{
		"one: bws://project/key",
		"two: bws://BARE_KEY",
		"three: passwordstate://42",
		"",
	}, "\n"))
	f, _ := countingFactory()
	service, err := f.NewValidateService(workspaceDir)
	if err != nil {
		t.Fatalf("NewValidateService: %v", err)
	}

	report, err := service.Validate(validate.ValidateParams{
		Selected: []diagnostic.Code{
			diagnostic.ExternalProviderUnavailable,
			diagnostic.InvalidExternalReference,
		},
	})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	var got []string
	for _, found := range report.Findings {
		got = append(got, string(found.Code)+" "+found.Environment+" "+found.Key)
	}
	want := []string{
		"EXTERNAL_PROVIDER_UNAVAILABLE  bws",
		"EXTERNAL_PROVIDER_UNAVAILABLE  passwordstate",
		"INVALID_EXTERNAL_REFERENCE prod TWO",
	}
	if !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
}

// TestCipherIsRequiredOnlyByConsumers verifies an unsupported cipher fails the
// features that decrypt and leaves packing and lazy validation usable.
func TestCipherIsRequiredOnlyByConsumers(t *testing.T) {
	t.Parallel()

	workspaceDir := testutil.CopyDir(t, minimalWorkspace)
	writeFile(t, filepath.Join(workspaceDir, "envx.yaml"), strings.Join([]string{
		"environments: [dev]",
		"secrets:",
		"  cipher: rot13",
		"projects:",
		"  app:",
		"    includes: [env/app]",
		"",
	}, "\n"))
	f, _ := countingFactory()

	if _, err := f.NewEnvService(workspaceDir); err == nil {
		t.Error("NewEnvService succeeded with an unsupported cipher")
	}
	if _, err := f.NewSecretsService(workspaceDir); err == nil {
		t.Error("NewSecretsService succeeded with an unsupported cipher")
	}
	if _, err := f.NewPackService(workspaceDir); err != nil {
		t.Errorf("NewPackService: %v", err)
	}
	service, err := f.NewValidateService(workspaceDir)
	if err != nil {
		t.Fatalf("NewValidateService: %v", err)
	}
	if _, err := service.Validate(validate.ValidateParams{}); err == nil {
		t.Error("Validate succeeded with an unsupported cipher")
	}
}

// TestMissingWorkspace verifies workspace constructors report a missing
// manifest.
func TestMissingWorkspace(t *testing.T) {
	t.Parallel()

	emptyDir := t.TempDir()
	f, _ := countingFactory()
	constructors := map[string]func(string) error{
		"NewEnvService": func(path string) error {
			_, err := f.NewEnvService(path)
			return err
		},
		"NewSecretsService": func(path string) error {
			_, err := f.NewSecretsService(path)
			return err
		},
		"NewPackService": func(path string) error {
			_, err := f.NewPackService(path)
			return err
		},
		"NewValidateService": func(path string) error {
			_, err := f.NewValidateService(path)
			return err
		},
		"NewEmitService": func(path string) error {
			_, err := f.NewEmitService(path)
			return err
		},
	}
	for name, construct := range constructors {
		if err := construct(emptyDir); !errors.Is(err, workspace.ErrNotFound) {
			t.Errorf("%s error = %v, want ErrNotFound", name, err)
		}
	}
}

// TestNewScaffoldServiceNeedsNoWorkspace verifies the scaffold service extracts the
// embedded templates without a manifest.
func TestNewScaffoldServiceNeedsNoWorkspace(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	svc, err := Factory{}.NewScaffoldService()
	if err != nil {
		t.Fatalf("NewScaffoldService: %v", err)
	}
	result, err := svc.Create(scaffold.CreateParams{
		Template:  scaffold.QuickStartTemplate,
		TargetDir: targetDir,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(result.Written) == 0 {
		t.Error("Create wrote no files")
	}
}
