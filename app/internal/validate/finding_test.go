package validate

import (
	"testing"

	"github.com/go-envx/envx/app/internal/secrets"
	"github.com/go-envx/envx/app/internal/shared/status"
)

// TestReportAddTallies verifies add appends findings and counts them by severity.
func TestReportAddTallies(t *testing.T) {
	t.Parallel()

	var r Report
	r.add(Finding{Severity: SeverityError})
	r.add(Finding{Severity: SeverityWarning})
	r.add(Finding{Severity: SeverityWarning})

	if len(r.Findings) != 3 {
		t.Errorf("Findings = %d, want 3", len(r.Findings))
	}
	if r.Errors != 1 {
		t.Errorf("Errors = %d, want 1", r.Errors)
	}
	if r.Warnings != 2 {
		t.Errorf("Warnings = %d, want 2", r.Warnings)
	}
}

// TestReportGrade verifies the pass/fail verdict: errors always fail, and
// warnings fail only under strict.
func TestReportGrade(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		errors     int
		warnings   int
		strict     bool
		wantFailed bool
	}{
		{name: "clean", wantFailed: false},
		{name: "warning default passes", warnings: 2, wantFailed: false},
		{name: "warning strict fails", warnings: 2, strict: true, wantFailed: true},
		{name: "error default fails", errors: 1, wantFailed: true},
		{name: "error strict fails", errors: 1, strict: true, wantFailed: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := Report{Errors: tc.errors, Warnings: tc.warnings}
			r.grade(tc.strict)
			if r.Failed != tc.wantFailed {
				t.Errorf("Failed = %v, want %v", r.Failed, tc.wantFailed)
			}
		})
	}
}

// TestReportRecordGradesByCode verifies record assigns a finding's severity from
// its code (default or configured override) and drops a code configured off.
func TestReportRecordGradesByCode(t *testing.T) {
	t.Parallel()

	// Defaults: orphan is an error, an unavailable key is a warning.
	var byDefault Report
	byDefault.record(Params{}, Finding{Code: status.SecretIsNotReferenced})
	byDefault.record(Params{}, Finding{Code: status.PrivateKeyIsUnavailable})
	if byDefault.Errors != 1 || byDefault.Warnings != 1 {
		t.Errorf("defaults graded %d errors %d warnings, want 1 and 1",
			byDefault.Errors, byDefault.Warnings)
	}

	// Overrides: suppress the orphan, promote the unavailable key to an error.
	params := Params{Severity: map[string]status.Severity{
		status.SecretIsNotReferenced:   status.Off,
		status.PrivateKeyIsUnavailable: status.Error,
	}}
	var overridden Report
	overridden.record(params, Finding{Code: status.SecretIsNotReferenced})
	overridden.record(params, Finding{Code: status.PrivateKeyIsUnavailable})
	if len(overridden.Findings) != 1 {
		t.Fatalf("off code should be dropped, got %d findings", len(overridden.Findings))
	}
	if overridden.Errors != 1 || overridden.Warnings != 0 {
		t.Errorf("overridden graded %d errors %d warnings, want 1 and 0",
			overridden.Errors, overridden.Warnings)
	}
}

// TestKeypairFinding verifies each private-key status maps onto the right
// finding code, and a valid key produces none. Severity is assigned later by
// record, so keypairFinding leaves it unset.
func TestKeypairFinding(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		status   secrets.PrivateKeyStatus
		wantOK   bool
		wantCode string
	}{
		{
			name:     "invalid maps to the invalid-key code",
			status:   secrets.PrivateKeyInvalid,
			wantOK:   true,
			wantCode: status.PrivateKeyIsInvalid,
		},
		{
			name:     "unavailable maps to the unavailable-key code",
			status:   secrets.PrivateKeyNotAvailable,
			wantOK:   true,
			wantCode: status.PrivateKeyIsUnavailable,
		},
		{name: "valid has no finding", status: secrets.PrivateKeyValid, wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			finding, ok := keypairFinding(secrets.KeypairMetadata{
				Group: "production", PrivateKeyStatus: tc.status,
			})
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if finding.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", finding.Code, tc.wantCode)
			}
			if finding.Key != "production" {
				t.Errorf("Key = %q, want production", finding.Key)
			}
		})
	}
}
