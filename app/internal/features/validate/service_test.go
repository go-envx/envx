package validate

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/features/secrets"
	"github.com/go-envx/envx/app/internal/shared/status"
)

// fakeEnvironment is an in-memory EnvService keyed by project.
type fakeEnvironment struct {
	entries map[string][]env.ExplanationEntry
	err     error
	calls   []env.ExplainParams
}

func (f *fakeEnvironment) Explain(
	params env.ExplainParams,
) (*env.ExplainResult, error) {
	f.calls = append(f.calls, params)
	if f.err != nil {
		return nil, f.err
	}
	return &env.ExplainResult{Entries: f.entries[params.Project]}, nil
}

// fakeStore is an in-memory SecretsService.
type fakeStore struct {
	stored   []secrets.StoredSecret
	missing  []string
	keypairs []secrets.KeypairMetadata
	err      error
}

func (f *fakeStore) StoredSecrets() ([]secrets.StoredSecret, error) {
	return f.stored, f.err
}

func (f *fakeStore) GroupsMissingPublicKey() ([]string, error) {
	return f.missing, f.err
}

func (f *fakeStore) ListKeypairs() ([]secrets.KeypairMetadata, error) {
	return f.keypairs, f.err
}

// danglingEntry is an explanation entry whose reference resolves to nothing.
func danglingEntry(key string) env.ExplanationEntry {
	return env.ExplanationEntry{
		Key: key,
		Resolution: env.Resolution{
			Severity: env.SeverityError,
			Code:     status.SecretReferenceNotFound,
			Message:  "no stored value for this reference",
		},
	}
}

// newTestService constructs a Service over the fakes and fails the test on error.
func newTestService(t *testing.T, params ServiceParams) *Service {
	t.Helper()
	service, err := NewService(params)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	return service
}

// TestNewServiceRequiresEnvService verifies construction rejects a missing
// env service.
func TestNewServiceRequiresEnvService(t *testing.T) {
	t.Parallel()

	if _, err := NewService(ServiceParams{}); err == nil {
		t.Fatal("NewService() accepted a nil env service")
	}
}

// TestServiceValidateReportsResolutionFindings verifies a non-OK resolution is
// recorded against its project and environment, and that every project is
// diagnosed against every declared environment.
func TestServiceValidateReportsResolutionFindings(t *testing.T) {
	t.Parallel()

	environment := &fakeEnvironment{entries: map[string][]env.ExplanationEntry{
		"api": {danglingEntry("PASSWORD")},
		"web": {{Key: "NAME", Resolution: env.Resolution{Severity: env.SeverityOK}}},
	}}
	service := newTestService(t, ServiceParams{
		EnvService:   environment,
		Projects:     []string{"api", "web"},
		Environments: []string{"development", "production"},
	})

	report, err := service.Validate(ValidateParams{})
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}

	if len(environment.calls) != 4 {
		t.Errorf("Explain calls = %d, want 4 (2 projects x 2 environments)",
			len(environment.calls))
	}
	if report.Errors != 2 || !report.Failed {
		t.Fatalf("report = %+v, want 2 errors and a failed run", report)
	}
	finding := report.Findings[0]
	if finding.Project != "api" || finding.Key != "PASSWORD" ||
		finding.Code != status.SecretReferenceNotFound {
		t.Errorf("finding = %+v, want a reference error on api PASSWORD", finding)
	}
}

// TestServiceValidatePassesOptionsAndMasks verifies the caller's setting overrides
// reach the diagnoser and the explanation never reveals plaintext.
func TestServiceValidatePassesOptionsAndMasks(t *testing.T) {
	t.Parallel()

	prefix := "APP_"
	environment := &fakeEnvironment{}
	service := newTestService(t, ServiceParams{
		EnvService:   environment,
		Projects:     []string{"api"},
		Environments: []string{"production"},
	})

	if _, err := service.Validate(ValidateParams{
		Options: env.Options{Prefix: &prefix},
	}); err != nil {
		t.Fatalf("Validate(): %v", err)
	}

	call := environment.calls[0]
	if call.Reveal {
		t.Error("validate must never reveal plaintext")
	}
	if call.Options.Prefix == nil || *call.Options.Prefix != prefix {
		t.Errorf("Options.Prefix = %v, want %q", call.Options.Prefix, prefix)
	}
}

// TestServiceValidateAppliesSeverityOverrides verifies the configured severity
// drops a finding turned off and the strict policy fails on a warning.
func TestServiceValidateAppliesSeverityOverrides(t *testing.T) {
	t.Parallel()

	environment := &fakeEnvironment{entries: map[string][]env.ExplanationEntry{
		"api": {danglingEntry("PASSWORD")},
	}}
	params := ServiceParams{
		EnvService:   environment,
		Projects:     []string{"api"},
		Environments: []string{"production"},
		Severity: map[string]status.Severity{
			status.SecretReferenceNotFound: status.Warn,
		},
	}

	relaxed, err := newTestService(t, params).Validate(ValidateParams{})
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if relaxed.Warnings != 1 || relaxed.Failed {
		t.Errorf("relaxed = %+v, want one warning that passes", relaxed)
	}

	strict, err := newTestService(t, params).Validate(ValidateParams{Strict: true})
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if !strict.Failed {
		t.Error("a warning must fail under strict")
	}

	params.Severity = map[string]status.Severity{
		status.SecretReferenceNotFound: status.Off,
	}
	off, err := newTestService(t, params).Validate(ValidateParams{Strict: true})
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if len(off.Findings) != 0 {
		t.Errorf("a check turned off must drop its findings: %+v", off.Findings)
	}
}

// TestServiceValidateReportsStoreFindings verifies the store-level findings are
// drawn from the store diagnoser, with an unreferenced value reported as an
// orphan.
func TestServiceValidateReportsStoreFindings(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		stored: []secrets.StoredSecret{
			{Group: "shared", Key: "leaked", Encrypted: false},
			{Group: "shared", Key: "legacy", Encrypted: true, AlgorithmMismatch: true},
		},
		missing: []string{"db"},
		keypairs: []secrets.KeypairMetadata{
			{Group: "shared", PrivateKeyStatus: secrets.PrivateKeyInvalid},
			{Group: "legacy", PrivateKeyStatus: secrets.PrivateKeyNotAvailable},
			{Group: "db", PrivateKeyStatus: secrets.PrivateKeyValid},
		},
	}
	service := newTestService(t, ServiceParams{
		EnvService:     &fakeEnvironment{},
		SecretsService: store,
		Projects:       []string{"api"},
		Environments:   []string{"production"},
	})

	report, err := service.Validate(ValidateParams{})
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}

	for _, code := range []string{
		status.SecretIsNotEncrypted,
		status.SecretAlgorithmMismatch,
		status.SecretIsNotReferenced,
		status.PublicKeyIsMissing,
		status.PrivateKeyIsInvalid,
		status.PrivateKeyIsUnavailable,
	} {
		if !hasCode(report, code) {
			t.Errorf("missing %s finding: %+v", code, report.Findings)
		}
	}
}

// TestServiceValidateStoreOnlySelectionSkipsMerge verifies selecting only a store
// check never calls the environment diagnoser.
func TestServiceValidateStoreOnlySelectionSkipsMerge(t *testing.T) {
	t.Parallel()

	environment := &fakeEnvironment{err: errors.New("merge must not run")}
	service := newTestService(t, ServiceParams{
		EnvService:     environment,
		SecretsService: &fakeStore{missing: []string{"db"}},
		Projects:       []string{"api"},
		Environments:   []string{"production"},
	})

	report, err := service.Validate(ValidateParams{
		Selected: map[string]bool{status.PublicKeyIsMissing: true},
	})
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if len(environment.calls) != 0 {
		t.Errorf("Explain calls = %d, want 0", len(environment.calls))
	}
	if !hasCode(report, status.PublicKeyIsMissing) {
		t.Errorf("missing the selected store finding: %+v", report.Findings)
	}
}

// TestServiceValidateNilStoreSkipsStoreChecks verifies a service without a store
// diagnoser runs only the per-environment checks.
func TestServiceValidateNilStoreSkipsStoreChecks(t *testing.T) {
	t.Parallel()

	service := newTestService(t, ServiceParams{
		EnvService:   &fakeEnvironment{},
		Projects:     []string{"api"},
		Environments: []string{"production"},
	})

	report, err := service.Validate(ValidateParams{})
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if len(report.Findings) != 0 {
		t.Errorf("Findings = %+v, want none", report.Findings)
	}
}

// TestServiceValidateReturnsDiagnoserErrors verifies a structural failure from
// either diagnoser aborts validation with the project and environment named.
func TestServiceValidateReturnsDiagnoserErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("boom")

	envFailure := newTestService(t, ServiceParams{
		EnvService:   &fakeEnvironment{err: want},
		Projects:     []string{"api"},
		Environments: []string{"production"},
	})
	if _, err := envFailure.Validate(ValidateParams{}); !errors.Is(err, want) {
		t.Errorf("environment err = %v, want %v", err, want)
	}

	storeFailure := newTestService(t, ServiceParams{
		EnvService:     &fakeEnvironment{},
		SecretsService: &fakeStore{err: want},
		Projects:       []string{"api"},
		Environments:   []string{"production"},
	})
	if _, err := storeFailure.Validate(ValidateParams{}); !errors.Is(err, want) {
		t.Errorf("store err = %v, want %v", err, want)
	}
}

// hasCode reports whether the report carries a finding with the given code.
func hasCode(report Report, code string) bool {
	for _, f := range report.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
