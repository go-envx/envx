package validate

import (
	"errors"
	"slices"
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/severity"
)

// TestValidateDiagnosesEveryProjectEnvironment verifies each project is
// explained, masked, in each declared environment with the caller's overrides.
func TestValidateDiagnosesEveryProjectEnvironment(t *testing.T) {
	t.Parallel()
	f := newFixture("web", "api")
	f.explainer.entries["api"] = []env.Entry{
		failing("PASSWORD", diagnostic.SecretReferenceNotFound, "secret://shared/pw"),
	}
	prefix := "APP_"

	report := f.validate(t, ValidateParams{Options: workspace.Settings{Prefix: &prefix}})

	type call struct {
		project     string
		environment string
	}
	var got []call
	for _, params := range f.explainer.calls {
		if params.Reveal {
			t.Error("Explain revealed plaintext")
		}
		if p := params.Query.Options.Prefix; p == nil || *p != prefix {
			t.Errorf("prefix = %v, want %q", p, prefix)
		}
		got = append(got, call{params.Query.Project, *params.Query.Options.Env})
	}
	want := []call{
		{"api", "development"},
		{"api", "production"},
		{"web", "development"},
		{"web", "production"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Explain calls = %v, want %v", got, want)
	}
	if report.Errors != 2 || !report.Failed {
		t.Fatalf("report = %+v, want 2 errors and a failed run", report)
	}
	found := report.Findings[0]
	if found.Project != "api" || found.Environment != "development" ||
		found.Key != "PASSWORD" || found.Level != severity.Error {
		t.Errorf("finding = %+v, want an api development PASSWORD error", found)
	}
}

// TestValidateGradesBySeverity verifies overrides regrade, disable, and enable
// findings and strict fails on a warning.
func TestValidateGradesBySeverity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		overrides map[string]string
		strict    bool
		errors    int
		warnings  int
		failed    bool
	}{
		{"defaults", nil, false, 2, 0, true},
		{
			"warn passes",
			map[string]string{"secret_reference_not_found": "warn"},
			false, 0, 2, false,
		},
		{
			"strict warn fails",
			map[string]string{"secret_reference_not_found": "warn"},
			true, 0, 2, true,
		},
		{
			"off drops",
			map[string]string{"secret_reference_not_found": "off"},
			true, 0, 0, false,
		},
		{
			"opt-in base check",
			map[string]string{"property_not_declared_in_base": "warn"},
			false, 2, 2, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture("api")
			f.ws.ValidateSeverities = tt.overrides
			overlayOnly := failing("PASSWORD", diagnostic.SecretReferenceNotFound)
			overlayOnly.DeclaredInBase = false
			f.explainer.entries["api"] = []env.Entry{overlayOnly}

			report := f.validate(t, ValidateParams{Strict: tt.strict})

			if report.Errors != tt.errors || report.Warnings != tt.warnings ||
				report.Failed != tt.failed {
				t.Errorf("report = %+v, want %d errors, %d warnings, failed %t",
					report, tt.errors, tt.warnings, tt.failed)
			}
		})
	}
}

// TestValidateReportsStoreFindings verifies each audit section maps onto its
// findings and a value referenced in any environment is not an orphan.
func TestValidateReportsStoreFindings(t *testing.T) {
	t.Parallel()
	f := newFixture("api")
	f.explainer.entries["api"] = []env.Entry{
		entry("TOKEN", "plain", "secret://Shared/token", `\secret://shared/escaped`),
	}
	f.inspector.audit = secrets.Audit{
		Secrets: []secrets.StoredSecret{
			stored("shared", "token", true, false),
			stored("shared", "escaped", true, false),
			stored("shared", "leaked", false, false),
			stored("shared", "foreign", true, true),
		},
		Keypairs: []secrets.KeypairMetadata{
			{Group: "shared", PrivateKeyStatus: secrets.PrivateKeyValid},
			{Group: "broken", PrivateKeyStatus: secrets.PrivateKeyInvalid},
			{Group: "remote", PrivateKeyStatus: secrets.PrivateKeyNotAvailable},
		},
		GroupsMissingPublicKey: []string{"orphaned"},
	}

	report := f.validate(t, ValidateParams{})

	type found struct {
		code diagnostic.Code
		key  string
	}
	var got []found
	for _, finding := range report.Findings {
		got = append(got, found{finding.Code, finding.Key})
	}
	want := []found{
		{diagnostic.PrivateKeyIsInvalid, "broken"},
		{diagnostic.PublicKeyIsMissing, "orphaned"},
		{diagnostic.SecretAlgorithmMismatch, "shared/foreign"},
		{diagnostic.SecretIsNotEncrypted, "shared/leaked"},
		{diagnostic.SecretIsNotReferenced, "shared/escaped"},
		{diagnostic.SecretIsNotReferenced, "shared/foreign"},
		{diagnostic.SecretIsNotReferenced, "shared/leaked"},
		{diagnostic.PrivateKeyIsUnavailable, "remote"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

// TestValidateReportsStoreProblemsOnce verifies a store-owned code carried by
// every reference to a value is reported once, by the audit, while
// resolution-owned codes stay per reference.
func TestValidateReportsStoreProblemsOnce(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		code  diagnostic.Code
		audit secrets.Audit
	}{
		{
			"plaintext",
			diagnostic.SecretIsNotEncrypted,
			secrets.Audit{Secrets: []secrets.StoredSecret{
				stored("shared", "pw", false, false),
			}},
		},
		{
			"algorithm mismatch",
			diagnostic.SecretAlgorithmMismatch,
			secrets.Audit{Secrets: []secrets.StoredSecret{
				stored("shared", "pw", true, true),
			}},
		},
		{
			"invalid private key",
			diagnostic.PrivateKeyIsInvalid,
			secrets.Audit{Keypairs: []secrets.KeypairMetadata{
				{Group: "shared", PrivateKeyStatus: secrets.PrivateKeyInvalid},
			}},
		},
		{
			"unavailable private key",
			diagnostic.PrivateKeyIsUnavailable,
			secrets.Audit{Keypairs: []secrets.KeypairMetadata{
				{Group: "shared", PrivateKeyStatus: secrets.PrivateKeyNotAvailable},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture("api", "web")
			for _, project := range []string{"api", "web"} {
				f.explainer.entries[project] = []env.Entry{
					failing("PASSWORD", tt.code, "secret://shared/pw"),
					failing("DSN", tt.code, "secret://shared/pw"),
					failing("MISSING", diagnostic.SecretReferenceNotFound, "secret://x/y"),
				}
			}
			f.inspector.audit = tt.audit

			report := f.validate(t, ValidateParams{})

			if n := count(report.Findings, tt.code); n != 1 {
				t.Errorf("%s reported %d times, want once: %v",
					tt.code, n, codes(report.Findings))
			}
			// One per project and environment.
			if n := count(report.Findings, diagnostic.SecretReferenceNotFound); n != 4 {
				t.Errorf("SECRET_REFERENCE_NOT_FOUND reported %d times, want 4", n)
			}
		})
	}
}

// TestValidateDeduplicatesFindings verifies a subject reported twice by an
// adapter yields one finding, and a store-owned code on an entry stays
// unreported when only resolution checks run.
func TestValidateDeduplicatesFindings(t *testing.T) {
	t.Parallel()
	t.Run("duplicate audit entries", func(t *testing.T) {
		t.Parallel()
		f := newFixture("api")
		leaked := stored("shared", "leaked", false, false)
		f.inspector.audit = secrets.Audit{
			Secrets:                []secrets.StoredSecret{leaked, leaked},
			GroupsMissingPublicKey: []string{"bare", "bare"},
		}
		report := f.validate(t, ValidateParams{})
		for _, code := range []diagnostic.Code{
			diagnostic.SecretIsNotEncrypted,
			diagnostic.SecretIsNotReferenced,
			diagnostic.PublicKeyIsMissing,
		} {
			if n := count(report.Findings, code); n != 1 {
				t.Errorf("%s reported %d times, want once", code, n)
			}
		}
	})
	t.Run("store code under resolution selection", func(t *testing.T) {
		t.Parallel()
		f := newFixture("api")
		f.explainer.entries["api"] = []env.Entry{
			failing("PASSWORD", diagnostic.SecretIsNotEncrypted, "secret://shared/pw"),
		}
		report := f.validate(t, ValidateParams{
			Selected: []diagnostic.Code{diagnostic.SecretReferenceNotFound},
		})
		if len(report.Findings) != 0 {
			t.Errorf("findings = %v, want none", codes(report.Findings))
		}
	})
}

// TestValidateReportsMaskedResolutionFindings verifies a resolution finding is
// reported even when a store-owned finding on the same value outranks it.
func TestValidateReportsMaskedResolutionFindings(t *testing.T) {
	t.Parallel()
	f := newFixture("api")
	f.ws.Environments = []string{"production"}
	mixed := failing("HOSTS", diagnostic.SecretIsNotEncrypted,
		"secret://shared/plain", "secret://shared/missing")
	def, _ := diagnostic.Lookup(diagnostic.SecretReferenceNotFound)
	mixed.Findings = append(mixed.Findings, diagnostic.Finding{
		Code: diagnostic.SecretReferenceNotFound, Level: def.Level, Message: "m",
	})
	f.explainer.entries["api"] = []env.Entry{mixed}

	report := f.validate(t, ValidateParams{
		Selected: []diagnostic.Code{diagnostic.SecretReferenceNotFound},
	})

	want := []diagnostic.Code{diagnostic.SecretReferenceNotFound}
	if got := codes(report.Findings); !slices.Equal(got, want) {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

// TestValidateCountsOverriddenReferences verifies a stored value referenced
// only by a namespace value a host value replaced is not an orphan.
func TestValidateCountsOverriddenReferences(t *testing.T) {
	t.Parallel()
	f := newFixture("api")
	overridden := entry("PASSWORD", "secret://shared/pw")
	f.explainer.entries["api"] = []env.Entry{overridden}
	f.inspector.audit = secrets.Audit{Secrets: []secrets.StoredSecret{
		stored("shared", "pw", true, false),
	}}

	report := f.validate(t, ValidateParams{})

	if len(report.Findings) != 0 {
		t.Errorf("findings = %v, want none", codes(report.Findings))
	}
}

// TestValidateSelectsChecks verifies a selection runs only its checks, opens
// only the inspectors they need, and audits only their sections.
func TestValidateSelectsChecks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		selected []diagnostic.Code
		envOpens int
		scopes   []secrets.AuditScope
		want     []diagnostic.Code
	}{
		{
			name:     "store values only",
			selected: []diagnostic.Code{diagnostic.SecretIsNotEncrypted},
			scopes:   []secrets.AuditScope{{Secrets: true}},
			want:     []diagnostic.Code{diagnostic.SecretIsNotEncrypted},
		},
		{
			name:     "keypairs only",
			selected: []diagnostic.Code{diagnostic.PrivateKeyIsInvalid},
			scopes:   []secrets.AuditScope{{Keypairs: true}},
			want:     []diagnostic.Code{diagnostic.PrivateKeyIsInvalid},
		},
		{
			name:     "missing public keys only",
			selected: []diagnostic.Code{diagnostic.PublicKeyIsMissing},
			scopes:   []secrets.AuditScope{{MissingPublicKeys: true}},
			want:     []diagnostic.Code{diagnostic.PublicKeyIsMissing},
		},
		{
			name:     "unreferenced values merge",
			selected: []diagnostic.Code{diagnostic.SecretIsNotReferenced},
			envOpens: 1,
			scopes:   []secrets.AuditScope{{Secrets: true}},
			want:     []diagnostic.Code{diagnostic.SecretIsNotReferenced},
		},
		{
			name:     "resolution only",
			selected: []diagnostic.Code{diagnostic.SecretReferenceNotFound},
			envOpens: 1,
			want:     []diagnostic.Code{diagnostic.SecretReferenceNotFound},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture("api")
			f.explainer.entries["api"] = []env.Entry{
				failing("MISSING", diagnostic.SecretReferenceNotFound, "secret://x/y"),
			}
			f.ws.Environments = []string{"production"}
			f.inspector.audit = secrets.Audit{
				Secrets: []secrets.StoredSecret{stored("shared", "leaked", false, false)},
				Keypairs: []secrets.KeypairMetadata{
					{Group: "shared", PrivateKeyStatus: secrets.PrivateKeyInvalid},
				},
				GroupsMissingPublicKey: []string{"bare"},
			}

			report := f.validate(t, ValidateParams{Selected: tt.selected})

			if f.envOpens != tt.envOpens {
				t.Errorf("explainer opened %d times, want %d", f.envOpens, tt.envOpens)
			}
			if f.secretsOpens != len(tt.scopes) {
				t.Errorf("inspector opened %d times, want %d",
					f.secretsOpens, len(tt.scopes))
			}
			if !slices.Equal(f.inspector.scopes, tt.scopes) {
				t.Errorf("audit scopes = %v, want %v", f.inspector.scopes, tt.scopes)
			}
			if got := codes(report.Findings); !slices.Equal(got, tt.want) {
				t.Errorf("findings = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidateSortsFindings verifies errors precede warnings, then store
// findings precede resolution findings, then code, project, and key order.
func TestValidateSortsFindings(t *testing.T) {
	t.Parallel()
	f := newFixture("web", "api")
	f.ws.Environments = []string{"production"}
	f.explainer.entries["web"] = []env.Entry{
		failing("TOKEN", diagnostic.SecretReferenceNotFound, "secret://shared/token"),
	}
	f.explainer.entries["api"] = []env.Entry{
		failing("PASSWORD", diagnostic.SecretReferenceNotFound, "secret://shared/pw"),
	}
	f.inspector.audit = secrets.Audit{
		Secrets: []secrets.StoredSecret{stored("shared", "leaked", false, false)},
		Keypairs: []secrets.KeypairMetadata{
			{Group: "shared", PrivateKeyStatus: secrets.PrivateKeyNotAvailable},
		},
	}
	f.ws.ValidateSeverities = map[string]string{"secret_is_not_referenced": "off"}

	report := f.validate(t, ValidateParams{})

	var got []string
	for _, found := range report.Findings {
		got = append(got, string(found.Code)+" "+found.Project+" "+found.Key)
	}
	want := []string{
		"SECRET_IS_NOT_ENCRYPTED  shared/leaked",
		"SECRET_REFERENCE_NOT_FOUND api PASSWORD",
		"SECRET_REFERENCE_NOT_FOUND web TOKEN",
		"PRIVATE_KEY_IS_UNAVAILABLE  shared",
	}
	if !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
}

// TestValidateFails verifies operational failures are errors, not findings.
func TestValidateFails(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	tests := []struct {
		name   string
		params ValidateParams
		setup  func(f *fixture)
		want   error
	}{
		{
			name:   "unknown check",
			params: ValidateParams{Selected: []diagnostic.Code{"NOPE"}},
			setup:  func(*fixture) {},
			want:   ErrUnknownCheck,
		},
		{
			name:  "explain failure",
			setup: func(f *fixture) { f.explainer.err = boom },
			want:  boom,
		},
		{
			name:  "audit failure",
			setup: func(f *fixture) { f.inspector.err = boom },
			want:  boom,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture("api")
			tt.setup(f)
			if _, err := f.service(t).Validate(tt.params); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestValidateOpenerFailures verifies a failing opener aborts the run.
func TestValidateOpenerFailures(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	ok := func() (Explainer, error) { return &fakeExplainer{}, nil }
	tests := []struct {
		name string
		deps Dependencies
	}{
		{"env", Dependencies{
			OpenEnv:     func() (Explainer, error) { return nil, boom },
			OpenSecrets: func() (Inspector, error) { return &fakeInspector{}, nil },
		}},
		{"secrets", Dependencies{
			OpenEnv:     ok,
			OpenSecrets: func() (Inspector, error) { return nil, boom },
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service, err := New(newFixture("api").ws, tt.deps)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Validate(ValidateParams{}); !errors.Is(err, boom) {
				t.Errorf("err = %v, want %v", err, boom)
			}
		})
	}
}

// TestNewRejectsInvalidConfiguration verifies construction requires both
// openers and valid severity overrides.
func TestNewRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()
	openEnv := func() (Explainer, error) { return &fakeExplainer{}, nil }
	openSecrets := func() (Inspector, error) { return &fakeInspector{}, nil }
	tests := []struct {
		name string
		ws   workspace.Workspace
		deps Dependencies
	}{
		{"no env opener", workspace.Workspace{}, Dependencies{OpenSecrets: openSecrets}},
		{"no secrets opener", workspace.Workspace{}, Dependencies{OpenEnv: openEnv}},
		{
			"invalid severity",
			workspace.Workspace{
				ValidateSeverities: map[string]string{"secret_is_not_encrypted": "loud"},
			},
			Dependencies{OpenEnv: openEnv, OpenSecrets: openSecrets},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := New(tt.ws, tt.deps); err == nil {
				t.Error("New succeeded, want an error")
			}
		})
	}
}
