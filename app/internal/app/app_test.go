package app

import (
	"errors"
	"os"
	"path/filepath"
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
	"github.com/go-envx/envx/app/internal/lib/hostenv"
	"github.com/go-envx/envx/app/internal/testutil"
)

// minimalWorkspace is the read-only composition fixture.
const minimalWorkspace = "testdata/minimal-workspace"

// countingFactory returns a factory whose snapshot source serves environment
// and counts captures.
func countingFactory(environment ...string) (Factory, *atomic.Int32) {
	captures := &atomic.Int32{}
	return Factory{Environment: func() hostenv.Snapshot {
		captures.Add(1)
		return hostenv.From(environment)
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
		t.Errorf("environment captured %d times, want 0", got)
	}
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("workspace has %d entries after construction, want 2", len(entries))
	}
}

// TestNewEnvServiceBindsOperationSnapshot verifies a revealed read decrypts through
// the store and keyring, and that the resolver reads private-key overrides from
// the snapshot the operation captured.
func TestNewEnvServiceBindsOperationSnapshot(t *testing.T) {
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
	if count := captures.Load(); count != 1 {
		t.Errorf("environment captured %d times, want 1", count)
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

// TestNewEnvServiceRegistersImplementedProviders verifies a declared provider
// with an adapter masks its references, while a declared provider without one
// passes through with a not-enabled warning.
func TestNewEnvServiceRegistersImplementedProviders(t *testing.T) {
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
		"two: bws://0b5c6f9e",
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
	if bws.Kind != value.Secret || bws.Value != "bws://0b5c6f9e" ||
		!bws.Diagnostic.IsZero() {
		t.Errorf("bws evaluation = %+v, want masked secret", bws)
	}
	pws := entries["THREE"].Evaluation
	if pws.Value != "passwordstate://42" ||
		pws.Diagnostic.Code != diagnostic.ExternalProviderNotEnabled {
		t.Errorf("passwordstate evaluation = %+v, want not-enabled warning", pws)
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
